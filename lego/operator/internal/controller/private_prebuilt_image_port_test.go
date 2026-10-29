/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controller

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const clickhouseExposed = `"8123/tcp":{},"9000/tcp":{},"9009/tcp":{}`

func prebuiltPrivateApp(image, pullSecret string) *appv1alpha1.App {
	return projectionApp(func(a *appv1alpha1.App) {
		a.Name = "clickhouse"
		a.Generation = 1
		a.Spec.Type = appv1alpha1.TypePrivateService
		a.Spec.PortMode = appv1alpha1.PortModeImageV1
		a.Spec.Image = image
		a.Spec.ExternalRegistryPullSecret = pullSecret
	})
}

// w6/095: a private service from a prebuilt public image publishes every TCP
// port the image declares, as a Git-built one does (w6/m148).
func TestPrivatePrebuiltImagePublishesDeclaredPorts(t *testing.T) {
	image := "clickhouse/clickhouse-server:23.8"
	r := &AppReconciler{Registry: "registry.test", HTTPClient: imageConfigRegistry(t, clickhouseExposed)}
	app := prebuiltPrivateApp(image, "")
	_, port, err := r.resolveImageNetwork(t.Context(), app, image)
	if err != nil {
		t.Fatal(err)
	}
	network := app.Status.ImageNetwork
	if port != 8123 || !reflect.DeepEqual(network.Ports, []int32{8123, 9000, 9009}) || network.Source != image {
		t.Fatalf("primary=%d network=%+v, want 8123 over [8123 9000 9009] from %s", port, network, image)
	}
	if !strings.HasPrefix(network.Image, "clickhouse/clickhouse-server@sha256:") {
		t.Fatalf("image = %q, want the digest-pinned prebuilt reference", network.Image)
	}
}

// A prebuilt image from a private registry is read with the workspace
// credential bound to the App: its Docker Hub entry, keyed docker.io rather
// than the API host, is exchanged once for a bearer token at the realm.
func TestPrivatePrebuiltImageReadsWithTheBoundRegistryCredential(t *testing.T) {
	const pullSecret = "reg-ext-clickhouse"
	basic := "Basic " + base64.StdEncoding.EncodeToString([]byte("bot:s3cret"))
	config := fmt.Sprintf(`{"architecture":"amd64","os":"linux","config":{"ExposedPorts":{%s}}}`, clickhouseExposed)
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(config)))
	manifest := fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"digest":%q}}`, configDigest)
	var tokenAuth []string
	httpClient := &http.Client{Transport: imageNetworkTransport(func(request *http.Request) (*http.Response, error) {
		respond := func(status int, body string, header http.Header) (*http.Response, error) {
			if header == nil {
				header = make(http.Header)
			}
			return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}, nil
		}
		if request.URL.Host == "auth.docker.io" {
			tokenAuth = append(tokenAuth, request.Header.Get("Authorization"))
			if request.Header.Get("Authorization") != basic {
				return respond(http.StatusUnauthorized, "", nil)
			}
			return respond(http.StatusOK, `{"token":"tok"}`, nil)
		}
		if request.Header.Get("Authorization") != "Bearer tok" {
			return respond(http.StatusUnauthorized, "", http.Header{"Www-Authenticate": {
				`Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:acme/clickhouse:pull"`,
			}})
		}
		if strings.HasSuffix(request.URL.Path, "/blobs/"+configDigest) {
			return respond(http.StatusOK, config, nil)
		}
		return respond(http.StatusOK, manifest, nil)
	})}
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	app := prebuiltPrivateApp("acme/clickhouse:23.8", pullSecret)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: pullSecret, Namespace: app.Namespace},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(
			`{"auths":{"docker.io":{"username":"bot","password":"s3cret"}}}`,
		)},
	}
	// The App namespace is outside the manager's cache: the Secret is only
	// visible to the uncached client.
	cached := fake.NewClientBuilder().WithScheme(scheme).Build()
	uncached := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	r := &AppReconciler{Client: cached, BuildClient: uncached, Registry: "registry.test", HTTPClient: httpClient}
	_, port, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image)
	if err != nil {
		t.Fatal(err)
	}
	if port != 8123 || !reflect.DeepEqual(app.Status.ImageNetwork.Ports, []int32{8123, 9000, 9009}) {
		t.Fatalf("primary=%d ports=%v, want 8123 over [8123 9000 9009]", port, app.Status.ImageNetwork.Ports)
	}
	if len(tokenAuth) != 1 || tokenAuth[0] != basic {
		t.Fatalf("token requests carried %q, want the bound credential", tokenAuth)
	}
}

// A missing credential Secret fails the read instead of silently going
// anonymous, which would publish a different image's ports or none.
func TestPrivatePrebuiltImageWithAMissingCredentialSecretFails(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	r := &AppReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(), Registry: "registry.test",
		HTTPClient: imageConfigRegistry(t, clickhouseExposed),
	}
	app := prebuiltPrivateApp("acme/clickhouse:23.8", "reg-ext-missing")
	if _, _, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image); err == nil || !strings.Contains(err.Error(), "reg-ext-missing") {
		t.Fatalf("err = %v, want the missing Secret named", err)
	}
}

// A credential for another host is never sent: kubelet would not use it for
// this image, so the read is anonymous, as the pull is.
func TestPrivatePrebuiltImageNeverSendsAnotherHostsCredential(t *testing.T) {
	var sent []string
	inner := imageConfigRegistry(t, clickhouseExposed)
	httpClient := &http.Client{Transport: imageNetworkTransport(func(request *http.Request) (*http.Response, error) {
		sent = append(sent, request.Header.Get("Authorization"))
		return inner.Transport.RoundTrip(request)
	})}
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	app := prebuiltPrivateApp("quay.io/acme/clickhouse:23.8", "reg-ext-ghcr")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "reg-ext-ghcr", Namespace: app.Namespace},
		Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{"ghcr.io":{"auth":"Ym90OnMzY3JldA=="}}}`)},
	}
	r := &AppReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build(), Registry: "registry.test",
		HTTPClient: httpClient,
	}
	if _, _, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image); err != nil {
		t.Fatal(err)
	}
	for _, header := range sent {
		if header != "" {
			t.Fatalf("sent Authorization %q to quay.io, want an anonymous read", header)
		}
	}
}

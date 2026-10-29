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
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Run through scripts/verify-ingress-tls.sh, which supplies a disposable cluster
// running the pinned Traefik with the repository's entrypoints and plugins.
func TestIngressTLSIsolationWithTraefik(t *testing.T) {
	kubeconfig := os.Getenv("BEX_TEST_TRAEFIK_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("run scripts/verify-ingress-tls.sh for the real Traefik provider")
	}
	ctx := context.Background()
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	scheme := newHostRedirectScheme()
	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	put := func(t *testing.T, obj client.Object) {
		t.Helper()
		if err := cl.Create(ctx, obj); err != nil {
			t.Fatal(err)
		}
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "tls-check-"}}
	put(t, ns)
	t.Cleanup(func() {
		if err := cl.Delete(ctx, ns); err != nil {
			t.Errorf("delete test namespace: %v", err)
		}
	})
	issue, roots := ingressTestCertificateIssuer(t)

	put(t, issue(t, "tls-check-fallback", "traefik", "*.onbex.test"))
	store := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"defaultCertificate": map[string]any{"secretName": "tls-check-fallback"}}}}
	store.SetGroupVersionKind(schema.GroupVersionKind{Group: "traefik.io", Version: "v1alpha1", Kind: "TLSStore"})
	store.SetName("default")
	store.SetNamespace("traefik")
	put(t, store)
	labels := map[string]string{"app": "tls-echo"}
	put(t, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "echo", Namespace: ns.Name}, Spec: appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{MatchLabels: labels}, Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: labels}, Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name: "echo", Image: "traefik/whoami:v1.11.0", Ports: []corev1.ContainerPort{{ContainerPort: 80}},
				ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(80)}}},
			}}}}}})
	put(t, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "echo", Namespace: ns.Name}, Spec: corev1.ServiceSpec{
		Selector: labels, Ports: []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt32(80)}}}})
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}, DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			_, port, _ := net.SplitHostPort(address)
			forward := os.Getenv("BEX_TEST_TRAEFIK_HTTP")
			if port == "443" {
				forward = os.Getenv("BEX_TEST_TRAEFIK_HTTPS")
			}
			return (&net.Dialer{}).DialContext(ctx, network, forward)
		}}
	t.Cleanup(transport.CloseIdleConnections)
	httpClient := &http.Client{Transport: transport, Timeout: 3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	await := func(t *testing.T, check func() error) {
		t.Helper()
		var last error
		for deadline := time.Now().Add(60 * time.Second); time.Now().Before(deadline); time.Sleep(250 * time.Millisecond) {
			if last = check(); last == nil {
				return
			}
		}
		t.Fatal(last)
	}
	serves := func(host string, status int) error {
		response, err := httpClient.Get("https://" + host + "/x?y=1")
		if err != nil {
			return err
		}
		defer func() { _ = response.Body.Close() }()
		body, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != status {
			return fmt.Errorf("%s: status %d, body %s, error %v", host, response.StatusCode, body, err)
		}
		if status == 200 && !strings.Contains(string(body), "X-Forwarded-Proto: https") {
			return fmt.Errorf("HTTPS did not reach the echo app: %s", body)
		}
		return nil
	}
	await(t, func() error {
		var deployment appsv1.Deployment
		if err := cl.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: "echo"}, &deployment); err != nil {
			return err
		}
		if deployment.Status.AvailableReplicas < 1 {
			return fmt.Errorf("waiting for echo fixture to become available")
		}
		return nil
	})
	for _, tc := range []struct {
		name  string
		ready []int
	}{
		{"platform-pending", []int{1, 2}},
		{"custom-pending", []int{0, 2}},
		{"two-customs", []int{2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hosts := []string{tc.name + ".onbex.test", "first-" + tc.name + ".example.test", "last-" + tc.name + ".example.test"}
			app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: tc.name, Namespace: ns.Name},
				Spec: appv1alpha1.AppSpec{Image: "traefik/whoami:v1.11.0", Hosts: hosts}}
			put(t, app)
			for _, i := range tc.ready {
				put(t, issue(t, appv1alpha1.TLSSecretName(app.Name, i == 0, hosts[i]), ns.Name, hosts[i]))
			}
			r := &AppReconciler{Client: cl, BuildClient: cl, Scheme: scheme, HTTPSRedirect: true}
			reconcile := func() {
				t.Helper()
				if err := r.reconcileIngress(ctx, app, hosts, "echo", 80, nil); err != nil {
					t.Fatal(err)
				}
			}
			reconcile()
			if tc.name == "platform-pending" {
				// Replay the old projection against the real provider. The ready
				// custom cert must fail verification with the missing reference first.
				var ing networkingv1.Ingress
				if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &ing); err != nil {
					t.Fatal(err)
				}
				ing.Spec.TLS = nil
				for i, host := range hosts {
					ing.Spec.TLS = append(ing.Spec.TLS, networkingv1.IngressTLS{Hosts: []string{host},
						SecretName: appv1alpha1.TLSSecretName(app.Name, i == 0, host)})
				}
				if err := cl.Update(ctx, &ing); err != nil {
					t.Fatal(err)
				}
				await(t, func() error {
					err := serves(hosts[1], 200)
					if _, ok := errors.AsType[x509.HostnameError](err); ok {
						return nil
					}
					return fmt.Errorf("old projection did not reproduce certificate mismatch: %v", err)
				})
				t.Log("old projection reproduced the custom-domain certificate mismatch")
				reconcile()
			}
			for _, i := range append([]int{0}, tc.ready...) {
				await(t, func() error { return serves(hosts[i], 200) })
				response, err := httpClient.Get("http://" + hosts[i] + "/x?y=1")
				if err != nil {
					t.Fatal(err)
				}
				_ = response.Body.Close()
				if response.StatusCode != 301 || response.Header.Get("Location") != "https://"+hosts[i]+"/x?y=1" {
					t.Fatalf("redirect changed: %d %s", response.StatusCode, response.Header.Get("Location"))
				}
			}
			t.Log("ready host certificates and wildcard fallback serve verified HTTPS; redirects preserve path/query")
		})
	}
}

func ingressTestCertificateIssuer(t *testing.T) (func(*testing.T, string, string, string) *corev1.Secret, *x509.CertPool) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Ingress TLS test CA"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}))
	issue := func(t *testing.T, name, namespace, host string) *corev1.Secret {
		t.Helper()
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
		if err != nil {
			t.Fatal(err)
		}
		leaf := &x509.Certificate{SerialNumber: serial, DNSNames: []string{host},
			NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, KeyUsage: x509.KeyUsageDigitalSignature}
		der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		private, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}, Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{corev1.TLSCertKey: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
				corev1.TLSPrivateKeyKey: pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: private})}}
	}
	return issue, roots
}

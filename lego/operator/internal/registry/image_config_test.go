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

package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
)

func imageMetadataFixture(t *testing.T) (*http.Client, string, map[string][]byte) {
	t.Helper()
	config := []byte(`{"architecture":"amd64","os":"linux","config":{"ExposedPorts":{"9009/tcp":{},"8123/tcp":{},"9000/tcp":{}}}}`)
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(config))
	manifest := []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"digest":%q}}`, configDigest))
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(manifest))
	routes := map[string][]byte{
		"/v2/owned/clickhouse/manifests/gen-1":       manifest,
		"/v2/owned/clickhouse/manifests/" + digest:   manifest,
		"/v2/owned/clickhouse/blobs/" + configDigest: config,
	}
	server := imageMetadataClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Basic fixture" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, ok := routes[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(body)
	}))
	return server, digest, routes
}

func TestReadImageConfigPinsManifestAndPrivatePorts(t *testing.T) {
	server, digest, _ := imageMetadataFixture(t)
	for _, reference := range []string{"gen-1", digest} {
		result, err := ReadImageConfig(context.Background(), server, "https://registry.test", "owned/clickhouse", reference, "Basic fixture")
		if err != nil {
			t.Fatal(err)
		}
		if result.Digest != digest || !reflect.DeepEqual(result.TCPPorts, []int32{8123, 9000, 9009}) {
			t.Fatalf("metadata = %+v", result)
		}
	}
}

func TestReadImageConfigResolvesPlatformIndex(t *testing.T) {
	server, digest, routes := imageMetadataFixture(t)
	routes["/v2/owned/clickhouse/manifests/gen-1"] = []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"digest":"sha256:unused","platform":{"os":"linux","architecture":"arm64"}},{"digest":%q,"platform":{"os":"linux","architecture":"amd64"}}]}`, digest))
	result, err := ReadImageConfig(context.Background(), server, "https://registry.test", "owned/clickhouse", "gen-1", "Basic fixture")
	if err != nil || result.Digest != digest {
		t.Fatalf("platform metadata = %+v, %v", result, err)
	}
}

func TestReadImageConfigFailsClosed(t *testing.T) {
	for _, scenario := range []string{"unauthorized", "wrong repository", "manifest tamper", "config tamper", "oversize", "invalid reference"} {
		t.Run(scenario, func(t *testing.T) {
			server, digest, routes := imageMetadataFixture(t)
			repo, reference, auth := "owned/clickhouse", digest, "Basic fixture"
			switch scenario {
			case "unauthorized":
				auth = ""
			case "wrong repository":
				repo = "another/clickhouse"
			case "invalid reference":
				reference = "../../blobs/key"
			case "manifest tamper":
				routes["/v2/owned/clickhouse/manifests/"+digest] = []byte(`{}`)
			case "config tamper":
				for path := range routes {
					if strings.Contains(path, "/blobs/") {
						routes[path] = []byte(`{}`)
					}
				}
			case "oversize":
				routes["/v2/owned/clickhouse/manifests/"+digest] = make([]byte, boundedhttp.MaxBodyBytes+1)
			}
			result, err := ReadImageConfig(context.Background(), server, "https://registry.test", repo, reference, auth)
			if err == nil || result.Digest != "" || len(result.TCPPorts) != 0 {
				t.Fatalf("failed-open metadata = %+v, %v", result, err)
			}
		})
	}
}

func TestReadImageConfigRefusesRedirect(t *testing.T) {
	visited := false
	client := imageMetadataClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host == "outside.test" {
			visited = true
			return
		}
		http.Redirect(w, r, "https://outside.test", http.StatusTemporaryRedirect)
	}))
	_, err := ReadImageConfig(context.Background(), client, "https://registry.test", "owned/clickhouse", "gen-1", "Basic fixture")
	if err == nil || visited {
		t.Fatalf("redirect followed=%v, error=%v", visited, err)
	}
}

// Exercise real HTTP request/response and redirect handling without opening a
// listener. A transport seam also keeps metadata tests independent of DNS.
type imageMetadataTransport func(*http.Request) (*http.Response, error)

func (f imageMetadataTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func imageMetadataClient(handler http.Handler) *http.Client {
	return &http.Client{Transport: imageMetadataTransport(func(r *http.Request) (*http.Response, error) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, r)
		return response.Result(), nil
	})}
}

func TestPrivateTCPPortBounds(t *testing.T) {
	for _, port := range []string{"0/tcp", "65536/tcp", "18012/tcp", "18013/tcp", "19099/tcp", "bad/tcp", "8123/sctp", "8123"} {
		if _, err := privateTCPPorts(map[string]json.RawMessage{port: json.RawMessage(`{}`)}); err == nil {
			t.Errorf("accepted %q", port)
		}
	}
	exposed := map[string]json.RawMessage{}
	for port := 1; port <= 76; port++ {
		exposed[fmt.Sprintf("%d/tcp", port)] = json.RawMessage(`{}`)
	}
	if _, err := privateTCPPorts(exposed); err == nil {
		t.Fatal("accepted 76 ports")
	}
	ports, err := privateTCPPorts(map[string]json.RawMessage{"8123/tcp": json.RawMessage(`{}`), "8123/udp": json.RawMessage(`{}`)})
	if err != nil || !reflect.DeepEqual(ports, []int32{8123}) {
		t.Fatalf("TCP-only ports=%v, error=%v", ports, err)
	}
}

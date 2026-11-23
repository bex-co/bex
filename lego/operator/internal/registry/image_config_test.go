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
	"errors"
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
	config := []byte(`{"architecture":"amd64","os":"linux","history":[{"created_by":"# debian.sh --arch 'amd64' out/ 'bookworm' '@1749513600'"}],"config":{"Env":["PYTHON_VERSION=3.14.8","PATH=/usr/local/bin:/usr/bin"],"ExposedPorts":{"9009/tcp":{},"8123/tcp":{},"9000/tcp":{}}}}`)
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
		if result.Digest != digest || result.PlatformDigest != digest || !reflect.DeepEqual(result.TCPPorts, []int32{8123, 9000, 9009}) {
			t.Fatalf("metadata = %+v", result)
		}
	}
}

func TestReadImageConfigResolvesPlatformIndex(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprintf("nested=%v", nested), func(t *testing.T) {
			server, platform, routes := imageMetadataFixture(t)
			index := []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"digest":"sha256:unused","platform":{"os":"linux","architecture":"arm64"}},{"digest":%q,"platform":{"os":"linux","architecture":"amd64"}}]}`, platform))
			top := fmt.Sprintf("sha256:%x", sha256.Sum256(index))
			routes["/v2/owned/clickhouse/manifests/"+top] = index
			if nested {
				index = []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"digest":%q,"platform":{"os":"linux","architecture":"amd64"}}]}`, top))
				top = fmt.Sprintf("sha256:%x", sha256.Sum256(index))
				routes["/v2/owned/clickhouse/manifests/"+top] = index
			}
			routes["/v2/owned/clickhouse/manifests/gen-1"] = index
			for _, reference := range []string{"gen-1", top} {
				result, err := ReadImageConfig(t.Context(), server, "https://registry.test", "owned/clickhouse", reference, "Basic fixture")
				if err != nil || result.Digest != top || result.PlatformDigest != platform || !reflect.DeepEqual(result.TCPPorts, []int32{8123, 9000, 9009}) {
					t.Fatalf("reference=%s metadata=%+v err=%v want top=%s platform=%s", reference, result, err, top, platform)
				}
			}
		})
	}
}

func TestReadImageConfigIndexStillVerifiesEveryFetchedDigest(t *testing.T) {
	for _, scenario := range []string{"outer index", "child index", "platform manifest", "config"} {
		t.Run(scenario, func(t *testing.T) {
			server, platform, routes := imageMetadataFixture(t)
			child := []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"digest":%q,"platform":{"os":"linux","architecture":"amd64"}}]}`, platform))
			childDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(child))
			routes["/v2/owned/clickhouse/manifests/"+childDigest] = child
			outer := []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"digest":%q,"platform":{"os":"linux","architecture":"amd64"}}]}`, childDigest))
			top := fmt.Sprintf("sha256:%x", sha256.Sum256(outer))
			routes["/v2/owned/clickhouse/manifests/"+top] = outer
			switch scenario {
			case "outer index":
				routes["/v2/owned/clickhouse/manifests/"+top] = []byte(`{}`)
			case "child index":
				routes["/v2/owned/clickhouse/manifests/"+childDigest] = []byte(`{}`)
			case "platform manifest":
				routes["/v2/owned/clickhouse/manifests/"+platform] = []byte(`{}`)
			case "config":
				for path := range routes {
					if strings.Contains(path, "/blobs/") {
						routes[path] = []byte(`{}`)
					}
				}
			}
			result, err := ReadImageConfig(t.Context(), server, "https://registry.test", "owned/clickhouse", top, "Basic fixture")
			if err == nil || !strings.Contains(err.Error(), "digest mismatch") || !reflect.DeepEqual(result, ImageConfig{}) {
				t.Fatalf("tampering returned partial metadata=%+v err=%v", result, err)
			}
		})
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
			if err == nil || !reflect.DeepEqual(result, ImageConfig{}) {
				t.Fatalf("failed-open metadata = %+v, %v", result, err)
			}
		})
	}
}

// mirrorRegistry is registry.k8s.io's shape (w32/080): the registry answers
// every manifest read with a 307 to target(path), a mirror on another host,
// and serves blobs itself. The registry asks for want as Authorization (a
// Bearer challenge when it is a token); mirror answers the mirror's reads.
// seen records every Authorization header the mirror host received.
func mirrorRegistry(want string, target func(path string) string, mirror http.HandlerFunc) (*http.Client, *[]string) {
	seen := &[]string{}
	return imageMetadataClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host == "mirror.test" {
			*seen = append(*seen, r.Header.Get("Authorization"))
			mirror(w, r)
			return
		}
		if r.URL.Path == "/token" {
			_, _ = w.Write([]byte(`{"token":"anonymous-token"}`))
			return
		}
		if r.Header.Get("Authorization") != want {
			w.Header().Set("WWW-Authenticate", `Bearer realm="https://registry.test/token",service="registry.test"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if strings.Contains(r.URL.Path, "/manifests/") {
			http.Redirect(w, r, target(r.URL.Path), http.StatusTemporaryRedirect)
			return
		}
		mirror(w, r)
	})), seen
}

// The mirror path is Artifact Registry's, with the registry's request id.
func artifactRegistryMirror(path string) string {
	return "https://mirror.test/v2/k8s-artifacts-prod/images/" + strings.TrimPrefix(path, "/v2/") + "?rid=fixture"
}

// servesFixture answers a mirror or blob read from the fixture's routes, the
// mirror's paths mapped back to the registry's.
func servesFixture(routes map[string][]byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if rest, ok := strings.CutPrefix(path, "/v2/k8s-artifacts-prod/images/"); ok {
			path = "/v2/" + rest
		}
		body, ok := routes[path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(body)
	}
}

// registry.k8s.io serves every manifest from a regional Artifact Registry
// mirror (w32/080): the read follows the redirect, pins the digest of the
// bytes it read, and never carries the registry's credential to the mirror.
func TestReadImageConfigFollowsManifestMirrorRedirect(t *testing.T) {
	for _, scenario := range []string{"anonymous", "bearer", "credentialed", "index"} {
		t.Run(scenario, func(t *testing.T) {
			_, digest, routes := imageMetadataFixture(t)
			auth, want := "", ""
			switch scenario {
			case "bearer":
				want = "Bearer anonymous-token"
			case "credentialed":
				auth, want = "Basic fixture", "Basic fixture"
			}
			pinned, platform := digest, digest
			if scenario == "index" {
				index := []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.list.v2+json","manifests":[{"digest":%q,"platform":{"os":"linux","architecture":"amd64"}}]}`, platform))
				pinned = fmt.Sprintf("sha256:%x", sha256.Sum256(index))
				routes["/v2/owned/clickhouse/manifests/gen-1"] = index
			}
			client, seen := mirrorRegistry(want, artifactRegistryMirror, servesFixture(routes))
			result, err := ReadImageConfig(t.Context(), client, "https://registry.test", "owned/clickhouse", "gen-1", auth)
			if err != nil || result.Digest != pinned || result.PlatformDigest != platform || !reflect.DeepEqual(result.TCPPorts, []int32{8123, 9000, 9009}) {
				t.Fatalf("metadata = %+v, %v; want digest %s platform %s", result, err, pinned, platform)
			}
			if len(*seen) == 0 {
				t.Fatal("the read never reached the mirror")
			}
			for _, header := range *seen {
				if header != "" {
					t.Fatalf("the mirror received Authorization %q", header)
				}
			}
		})
	}
}

// A manifest redirect is followed only within redirectPolicy, and a redirect
// target's own challenge is never exchanged: its realm would receive the
// registry's Basic credential. Each refusal is the registry's definitive answer.
func TestReadImageConfigRefusesUnsafeManifestRedirect(t *testing.T) {
	for _, tc := range []struct {
		name, scenario, auth string
		status               int
	}{
		{"plaintext", "plaintext", "", http.StatusTemporaryRedirect},
		{"loop", "loop", "", http.StatusTemporaryRedirect},
		{"anonymous mirror challenge", "mirror challenge", "", http.StatusUnauthorized},
		{"credentialed mirror challenge", "mirror challenge", "Basic fixture", http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, routes := imageMetadataFixture(t)
			target := artifactRegistryMirror
			switch tc.scenario {
			case "plaintext":
				target = func(path string) string { return "http://mirror.test" + path }
			case "loop":
				target = func(path string) string { return "https://registry.test" + path }
			}
			tokenRequested := false
			client, seen := mirrorRegistry(tc.auth, target, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					tokenRequested = true
					_, _ = w.Write([]byte(`{"token":"mirror-token"}`))
					return
				}
				if r.URL.Host == "mirror.test" {
					w.Header().Set("WWW-Authenticate", `Bearer realm="https://mirror.test/token",service="mirror.test"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				servesFixture(routes)(w, r)
			})
			result, err := ReadImageConfig(t.Context(), client, "https://registry.test", "owned/clickhouse", "gen-1", tc.auth)
			var refused *RefusedError
			if !errors.As(err, &refused) || refused.Status != tc.status || result.Digest != "" {
				t.Fatalf("metadata = %+v, %v; want a refusal with status %d", result, err, tc.status)
			}
			if tokenRequested {
				t.Fatal("the read exchanged a credential at the redirect target's realm")
			}
			for _, header := range *seen {
				if header != "" {
					t.Fatalf("the mirror received Authorization %q", header)
				}
			}
		})
	}
}

// Every hop of a redirect still dials through the shared transport's address
// guard: a registry cannot redirect the operator to the metadata endpoint.
func TestReadImageConfigRedirectKeepsTheDialGuard(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://169.254.169.254/latest/meta-data/", http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	client := boundedhttp.New(boundedhttp.RequestTimeout)
	client.Transport.(*http.Transport).TLSClientConfig = srv.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	_, err := ReadImageConfig(t.Context(), client, srv.URL, "owned/clickhouse", "gen-1", "")
	if err == nil || !strings.Contains(err.Error(), "blocked address") {
		t.Fatalf("redirect to the metadata endpoint: %v, want the dial guard's refusal", err)
	}
}

// Docker Hub, GHCR, ECR and GCR serve config blobs through a redirect to
// storage (w8/201): follow it over https, without the registry credential.
func TestReadImageConfigFollowsBlobStorageRedirect(t *testing.T) {
	for _, scenario := range []string{"storage", "anonymous", "bearer", "plaintext", "loop", "tampered"} {
		t.Run(scenario, func(t *testing.T) {
			_, digest, routes := imageMetadataFixture(t)
			auth := "Basic fixture"
			want := auth
			switch scenario {
			case "anonymous":
				auth, want = "", ""
			case "bearer":
				auth, want = "", "Bearer anonymous-token"
			}
			leaked := false
			client := imageMetadataClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Host == "storage.test" {
					leaked = leaked || r.Header.Get("Authorization") != ""
					body := routes["/v2/owned/clickhouse/blobs/"+strings.TrimPrefix(r.URL.Path, "/")]
					if scenario == "tampered" {
						body = []byte(`{"architecture":"amd64","os":"linux"}`)
					}
					_, _ = w.Write(body)
					return
				}
				if r.URL.Path == "/token" {
					_, _ = w.Write([]byte(`{"token":"anonymous-token"}`))
					return
				}
				if r.Header.Get("Authorization") != want {
					w.Header().Set("WWW-Authenticate", `Bearer realm="https://registry.test/token",service="registry.test"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if blob, ok := strings.CutPrefix(r.URL.Path, "/v2/owned/clickhouse/blobs/"); ok {
					target := "https://storage.test/" + blob + "?X-Amz-Signature=fixture"
					switch scenario {
					case "plaintext":
						target = "http://storage.test/" + blob
					case "loop":
						target = r.URL.String()
					}
					http.Redirect(w, r, target, http.StatusTemporaryRedirect)
					return
				}
				body, ok := routes[r.URL.Path]
				if !ok {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				_, _ = w.Write(body)
			}))
			result, err := ReadImageConfig(context.Background(), client, "https://registry.test", "owned/clickhouse", "gen-1", auth)
			if leaked {
				t.Fatal("registry credential followed the storage redirect")
			}
			switch scenario {
			case "storage", "anonymous", "bearer":
				if err != nil || result.Digest != digest || !reflect.DeepEqual(result.TCPPorts, []int32{8123, 9000, 9009}) {
					t.Fatalf("metadata = %+v, %v", result, err)
				}
			case "plaintext", "loop":
				// Definitive, since a retry meets the same redirect: a refused
				// hop, or the registry's 401 to a hop without its credential.
				var refused *RefusedError
				if !errors.As(err, &refused) || result.Digest != "" {
					t.Fatalf("failed-open metadata = %+v, %v; want a refusal", result, err)
				}
			default:
				if err == nil || result.Digest != "" {
					t.Fatalf("failed-open metadata = %+v, %v", result, err)
				}
			}
		})
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

// The label read shares ReadImageConfig's verified fetch but not its port
// rules: an image declaring an invalid port still reports its provenance
// (w5/m148) and what its native-init does (w32/m575).
func TestReadImageLabelsReadsTheLabelsWithoutPortRules(t *testing.T) {
	config := []byte(`{"architecture":"amd64","os":"linux","config":{"ExposedPorts":{"70000/tcp":{}},"Labels":{"org.opencontainers.image.revision":"0123456789abcdef0123456789abcdef01234567","co.bex.native-init.announces":"true"}}}`)
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(config))
	manifest := []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"digest":%q}}`, configDigest))
	routes := map[string][]byte{
		"/v2/owned/web/manifests/gen-1":       manifest,
		"/v2/owned/web/blobs/" + configDigest: config,
	}
	server := imageMetadataClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := routes[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(body)
	}))
	ctx := context.Background()
	if _, err := ReadImageConfig(ctx, server, "https://registry.test", "owned/web", "gen-1", ""); err == nil {
		t.Fatal("fixture must declare a port ReadImageConfig refuses")
	}
	got, err := ReadImageLabels(ctx, server, "https://registry.test", "owned/web", "gen-1", "")
	if err != nil || got[RevisionLabel] != "0123456789abcdef0123456789abcdef01234567" || got["co.bex.native-init.announces"] != "true" || len(got) != 2 {
		t.Fatalf("ReadImageLabels = %v, %v", got, err)
	}
	routes["/v2/owned/web/blobs/"+configDigest] = []byte("tampered")
	if _, err := ReadImageLabels(ctx, server, "https://registry.test", "owned/web", "gen-1", ""); err == nil {
		t.Fatal("a config blob that does not match its digest must be refused")
	}
}

// A Basic credential is exchanged for a token only when the registry asks for
// a Bearer token; a Basic challenge to a Basic credential is a plain failure.
func TestExchangesForBearer(t *testing.T) {
	for _, tc := range []struct {
		authorization, challenge string
		want                     bool
	}{
		{"", `Bearer realm="https://auth.test/token"`, true},
		{"", `Basic realm="zot"`, true},
		{"Basic Ym90OnB3", `Bearer realm="https://auth.test/token"`, true},
		{"Basic Ym90OnB3", `Basic realm="zot"`, false},
		{"Bearer tok", `Bearer realm="https://auth.test/token"`, false},
	} {
		if got := exchangesForBearer(tc.authorization, tc.challenge); got != tc.want {
			t.Errorf("exchangesForBearer(%q, %q) = %v, want %v", tc.authorization, tc.challenge, got, tc.want)
		}
	}
}

func TestReadImageRuntimeMetadataVerifiesTheConfig(t *testing.T) {
	client, digest, routes := imageMetadataFixture(t)
	env, history, err := ReadImageRuntimeMetadata(t.Context(), client, "https://registry.test", "owned/clickhouse", digest, "Basic fixture")
	if err != nil || !reflect.DeepEqual(env, []string{"PYTHON_VERSION=3.14.8", "PATH=/usr/local/bin:/usr/bin"}) || len(history) != 1 || history[0] != "# debian.sh --arch 'amd64' out/ 'bookworm' '@1749513600'" {
		t.Fatalf("environment = %v, %v", env, err)
	}
	for path, body := range routes {
		if strings.Contains(path, "/blobs/") {
			routes[path] = []byte(strings.ReplaceAll(string(body), "3.14.8", "3.14.9"))
		}
	}
	if env, _, err := ReadImageRuntimeMetadata(t.Context(), client, "https://registry.test", "owned/clickhouse", digest, "Basic fixture"); err == nil || env != nil {
		t.Fatalf("accepted tampered image environment: %v, %v", env, err)
	}
}

// archFixture serves one single-platform image whose config declares arch,
// tagged "single", and an index tagged "index" that lists it under arch. An
// arm64 index lists an unresolvable amd64 entry first, so selection must skip
// it; an amd64 index is amd64-only.
func archFixture(t *testing.T, arch string) (*http.Client, string) {
	t.Helper()
	config := []byte(fmt.Sprintf(`{"architecture":%q,"os":"linux","config":{"ExposedPorts":{"8080/tcp":{}}}}`, arch))
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(config))
	manifest := []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"digest":%q}}`, configDigest))
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(manifest))
	entries := fmt.Sprintf(`{"digest":%q,"platform":{"os":"linux","architecture":%q}}`, digest, arch)
	if arch == "arm64" {
		entries = `{"digest":"sha256:` + strings.Repeat("0", 64) + `","platform":{"os":"linux","architecture":"amd64"}},` + entries
	}
	routes := map[string][]byte{
		"/v2/owned/app/manifests/single":      manifest,
		"/v2/owned/app/manifests/" + digest:   manifest,
		"/v2/owned/app/manifests/index":       []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[` + entries + `]}`),
		"/v2/owned/app/blobs/" + configDigest: config,
	}
	return imageMetadataClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := routes[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(body)
	})), digest
}

// w2/m293 t007: a non-default hosting platform resolves and verifies its own
// architecture, and refuses another with a message naming the expected one.
func TestReadImageConfigHonorsTheHostingPlatform(t *testing.T) {
	arm64 := Platform{OS: "linux", Architecture: "arm64"}
	arm, armDigest := archFixture(t, "arm64")
	for _, reference := range []string{"single", "index"} {
		result, err := readImageConfig(t.Context(), arm, "https://registry.test", "owned/app", reference, "", arm64)
		if err != nil || result.PlatformDigest != armDigest || !reflect.DeepEqual(result.TCPPorts, []int32{8080}) {
			t.Fatalf("arm64 %s: metadata=%+v err=%v", reference, result, err)
		}
	}
	amd, _ := archFixture(t, "amd64")
	for reference, want := range map[string]string{"single": "image config is not Linux/arm64", "index": "image has no Linux/arm64 manifest"} {
		result, err := readImageConfig(t.Context(), amd, "https://registry.test", "owned/app", reference, "", arm64)
		if err == nil || err.Error() != want || !reflect.DeepEqual(result, ImageConfig{}) {
			t.Fatalf("amd64-only %s under arm64: metadata=%+v err=%v want %q", reference, result, err, want)
		}
	}
	// The default platform still refuses the arm64 image, naming Linux/amd64,
	// and the package default is production's.
	if _, err := readImageConfig(t.Context(), arm, "https://registry.test", "owned/app", "single", "", DefaultPlatform); err == nil || err.Error() != "image config is not Linux/amd64" {
		t.Fatalf("default platform accepted arm64: %v", err)
	}
	if hostingPlatform != DefaultPlatform || DefaultPlatform != (Platform{OS: "linux", Architecture: "amd64"}) {
		t.Fatalf("hosting platform default = %+v", hostingPlatform)
	}
}

func TestParsePlatform(t *testing.T) {
	for value, want := range map[string]Platform{
		"":            DefaultPlatform,
		"linux/amd64": DefaultPlatform,
		"linux/arm64": {OS: "linux", Architecture: "arm64"},
	} {
		if got, err := ParsePlatform(value); err != nil || got != want {
			t.Fatalf("ParsePlatform(%q) = %+v, %v", value, got, err)
		}
	}
	for _, value := range []string{"amd64", "arm64", "linux", "linux/", "windows/amd64", "linux/arm/v7", "Linux/amd64", " linux/amd64"} {
		if _, err := ParsePlatform(value); err == nil || !strings.Contains(err.Error(), "linux/amd64 or linux/arm64") {
			t.Fatalf("ParsePlatform(%q) err = %v", value, err)
		}
	}
}

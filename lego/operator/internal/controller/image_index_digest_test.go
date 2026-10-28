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
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// The metadata reader selects amd64 config ports, while the deployed reference
// and release digest retain the index a client can pin independently of a node.
func TestWebImageIndexDigestSurvivesPinningAndDeployRecording(t *testing.T) {
	config := []byte(`{"architecture":"amd64","os":"linux","config":{"ExposedPorts":{"8080/tcp":{}}}}`)
	hash := func(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }
	configDigest := hash(config)
	manifest := []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"digest":%q}}`, configDigest))
	platformDigest := hash(manifest)
	index := []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"digest":%q,"platform":{"os":"linux","architecture":"amd64"}},{"digest":%q,"platform":{"os":"linux","architecture":"arm64"}}]}`, platformDigest, "sha256:"+strings.Repeat("a", 64)))
	indexDigest := hash(index)
	for _, reference := range []string{"0.9.2", indexDigest} {
		t.Run(reference, func(t *testing.T) {
			routes := map[string][]byte{
				"/v2/ealen/echo-server/manifests/0.9.2":             index,
				"/v2/ealen/echo-server/manifests/" + indexDigest:    index,
				"/v2/ealen/echo-server/manifests/" + platformDigest: manifest,
				"/v2/ealen/echo-server/blobs/" + configDigest:       config,
			}
			requests := 0
			r := &AppReconciler{HTTPClient: &http.Client{Transport: imageNetworkTransport(func(req *http.Request) (*http.Response, error) {
				requests++
				body, ok := routes[req.URL.Path]
				if !ok {
					return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("not found")), Header: http.Header{}}, nil
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: http.Header{}}, nil
			})}}
			app := webImageApp(1)
			separator := ":"
			if strings.HasPrefix(reference, "sha256:") {
				separator = "@"
			}
			app.Spec.Image = "docker.io/ealen/echo-server" + separator + reference
			image, port, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image)
			if err != nil || image != "docker.io/ealen/echo-server@"+indexDigest || port != 8080 {
				t.Fatalf("image=%s port=%d err=%v", image, port, err)
			}
			app.Status.Image = image
			r.recordImageDigest(t.Context(), app, nil)
			if app.Status.ImageDigest == nil || app.Status.ImageDigest.Digest != indexDigest || app.Status.ImageDigest.Digest == platformDigest {
				t.Fatalf("deploy digest=%+v, want requested index", app.Status.ImageDigest)
			}
			readRequests := requests
			if _, _, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image); err != nil {
				t.Fatal(err)
			}
			if requests != readRequests || app.Status.ImageNetwork.Image != image {
				t.Fatal("same release re-resolved index")
			}
			if reference == "0.9.2" {
				// A moved tag is resolved once for the next release; the old digest record
				// is never carried into that new generation.
				moved := []byte(strings.TrimSuffix(string(index), "}") + `,"annotations":{"version":"next"}}`)
				nextDigest := hash(moved)
				routes["/v2/ealen/echo-server/manifests/0.9.2"] = moved
				app.Status.ReleaseGeneration = 2
				app.Generation = 2
				image, _, err = r.resolveImageNetwork(t.Context(), app, app.Spec.Image)
				if err != nil || image != "docker.io/ealen/echo-server@"+nextDigest || requests != 2*readRequests {
					t.Fatalf("next image=%s requests=%d err=%v", image, requests, err)
				}
				app.Status.Image = image
				r.recordImageDigest(t.Context(), app, nil)
				if app.Status.ImageDigest.Generation != 2 || app.Status.ImageDigest.Digest != nextDigest {
					t.Fatalf("new generation kept prior digest: %+v", app.Status.ImageDigest)
				}
			}
		})
	}
}

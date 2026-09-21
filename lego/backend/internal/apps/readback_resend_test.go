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

package apps

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w9/070. One invariant, stated once: **whatever a service reads back must be
// re-sendable as a create**. That is what `services create --from` does, and
// breaking it has now cost three separate defects, each a different field:
//
//   - w4/052   — the build strategy (`serviceDetails.runtime`), fixed by w9/m93
//   - w9/068   — `serviceDetails.maintenanceMode`, refused on a free plan merely
//     for being present, though every read emits it
//   - w9/069   — top-level `branch`, published on a prebuilt-image service that
//     has no repo, and refused by the create that publishes it
//
// The earlier guards were per-field or per-block: m93's covers build-strategy
// shapes, and w9/068's echoes only `serviceDetails`, which is exactly why it
// could not see 069 one level up. This one derives the echoable field set from
// `createServiceRequest` itself, so a create-settable field added tomorrow is
// covered without anyone remembering to extend a list.
//
// It is deliberately a DRY RUN: the subject is whether the request is
// ACCEPTED, not what it builds.

// createSettableKeys is every top-level JSON key the create API accepts,
// derived from the request struct rather than hand-listed.
func createSettableKeys(t *testing.T) map[string]bool {
	t.Helper()
	keys := map[string]bool{}
	typ := reflect.TypeOf(createServiceRequest{})
	for i := range typ.NumField() {
		tag := typ.Field(i).Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name != "" && name != "-" {
			keys[name] = true
		}
	}
	if !keys["branch"] || !keys["serviceDetails"] {
		t.Fatalf("createServiceRequest keys look wrong: %v", keys)
	}
	return keys
}

func TestEveryServiceShapeReadsBackReSendable(t *testing.T) {
	settable := createSettableKeys(t)
	// Server-owned or identity fields a client never re-sends: the clone gets a
	// new name, and these describe the source, not the request.
	skip := map[string]bool{"name": true, "ownerId": true, "environmentId": true}

	repoApp := func(mut func(*appv1alpha1.App)) *appv1alpha1.App {
		a := sampleApp("subject")
		a.Spec.Image = ""
		a.Spec.Repo = "https://github.com/bex-co/example"
		a.Spec.Branch = "main"
		a.Spec.Replicas = 1
		// A native runtime always carries these — create requires them, so a
		// service without them is not a state the API can produce.
		a.Spec.BuildCommand = "go build ./..."
		a.Spec.StartCommand = "./server"
		if mut != nil {
			mut(a)
		}
		return a
	}
	imageApp := func(mut func(*appv1alpha1.App)) *appv1alpha1.App {
		a := sampleApp("subject") // Image set, no Repo.
		a.Spec.Replicas = 1
		if mut != nil {
			mut(a)
		}
		return a
	}

	for _, tc := range []struct {
		name string
		app  *appv1alpha1.App
	}{
		{"free web service, repo-backed", repoApp(func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeWebService
			a.Spec.Tier = "free"
			a.Spec.Runtime = "go"
			a.Spec.Builder = "native"
		})},
		{"paid web service, repo-backed", repoApp(func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeWebService
			a.Spec.Tier = "starter"
			a.Spec.Runtime = "go"
			a.Spec.Builder = "native"
		})},
		{"prebuilt image web service", imageApp(func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeWebService
			a.Spec.Runtime = "image"
		})},
		{"docker web service", repoApp(func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeWebService
			a.Spec.Runtime = "docker"
			a.Spec.Builder = "dockerfile"
			a.Spec.StartCommand = "./server"
		})},
		{"docker cron job", repoApp(func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeCronJob
			a.Spec.Runtime = "docker"
			a.Spec.Builder = "dockerfile"
			a.Spec.Schedule = "*/15 * * * *"
			a.Spec.Command = "./job"
		})},
		{"native cron job", repoApp(func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeCronJob
			a.Spec.Runtime = "go"
			a.Spec.Builder = "native"
			a.Spec.Schedule = "*/15 * * * *"
			a.Spec.Command = "./job"
		})},
		{"static site", repoApp(func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeStaticSite
			a.Spec.PublishPath = "dist"
		})},
		{"private service", repoApp(func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypePrivateService
			a.Spec.Runtime = "go"
			a.Spec.Builder = "native"
		})},
		{"background worker", repoApp(func(a *appv1alpha1.App) {
			a.Spec.Type = appv1alpha1.TypeBackgroundWorker
			a.Spec.Runtime = "go"
			a.Spec.Builder = "native"
		})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(nil, tc.app)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/subject", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET = %d: %s", rec.Code, rec.Body.String())
			}
			var read map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &read); err != nil {
				t.Fatal(err)
			}

			clone := map[string]any{"name": "subject-clone", "dryRun": true}
			// The one spelling the client itself translates: a read publishes a
			// prebuilt image as top-level `imagePath`, while create takes
			// Render's nested `image` object (pkg/service/clone.go does the same
			// mapping). Everything else is echoed verbatim.
			if path, ok := read["imagePath"].(string); ok && path != "" {
				clone["image"] = map[string]any{"imagePath": path, "ownerId": ""}
			}
			echoed := make([]string, 0, len(read))
			for key, value := range read {
				if !settable[key] || skip[key] || value == nil {
					continue
				}
				clone[key] = value
				echoed = append(echoed, key)
			}
			body, err := json.Marshal(clone)
			if err != nil {
				t.Fatal(err)
			}
			rec = httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(string(body))))
			if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
				t.Fatalf("re-sending this shape's own read as a create = %d: %s\nechoed keys: %v",
					rec.Code, rec.Body.String(), echoed)
			}
		})
	}
}

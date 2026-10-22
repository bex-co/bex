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
	"context"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/graphql-go/graphql"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// renderServerPortSchema reads Render's own `serverPort` schema from the
// vendored contract, so the shape below is checked against Render rather than
// against a copy of it.
func renderServerPortSchema(t *testing.T) (required []string, protocols []string, typesRequiring []string) {
	t.Helper()
	raw, err := os.ReadFile("../api/openapi/render-public-api-1.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Required   []string `json:"required"`
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	port := doc.Components.Schemas["serverPort"]
	for name, schema := range doc.Components.Schemas {
		if slices.Contains(schema.Required, "openPorts") {
			typesRequiring = append(typesRequiring, name)
		}
	}
	slices.Sort(typesRequiring)
	return port.Required, port.Properties["protocol"].Enum, typesRequiring
}

// serviceDetails.openPorts is Render's required serverPort[] on web and private
// services and absent elsewhere (w6/m148). An image-port service reports the
// serving set — [] while discovery is pending, never a guessed 3000 — and REST,
// MCP (the same renderServiceDetails) and GraphQL (AppView.OpenPorts) agree.
func TestRenderServiceDetailsOpenPorts(t *testing.T) {
	required, protocols, typesRequiring := renderServerPortSchema(t)
	if !reflect.DeepEqual(typesRequiring, []string{"privateServiceDetails", "webServiceDetails"}) {
		t.Fatalf("Render requires openPorts on %v; this test pins web and private only", typesRequiring)
	}
	image := func(mutate func(*appv1alpha1.App)) *appv1alpha1.App {
		a := sampleApp("ch")
		a.Spec.Type, a.Spec.PortMode = appv1alpha1.TypePrivateService, appv1alpha1.PortModeImageV1
		if mutate != nil {
			mutate(a)
		}
		return a
	}
	serving := &appv1alpha1.ImageNetworkStatus{Image: "a", Revision: "r1", PrimaryPort: 8123, Ports: []int32{8123, 9000, 9009}}
	for _, tc := range []struct {
		name string
		app  *appv1alpha1.App
		want []any // nil = absent
	}{
		{"configured web", func() *appv1alpha1.App { a := sampleApp("web"); a.Spec.Port = 8080; return a }(), []any{8080}},
		{"configured private", func() *appv1alpha1.App { a := sampleApp("p"); a.Spec.Type = appv1alpha1.TypePrivateService; return a }(), []any{10000}},
		{"image pending", image(nil), []any{}},
		{"image serving", image(func(a *appv1alpha1.App) { a.Status.ServingNetwork = serving }), []any{8123, 9000, 9009}},
		{"image replacing keeps the serving set", image(func(a *appv1alpha1.App) {
			a.Status.ServingNetwork = serving
			a.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "b", Revision: "r2", PrimaryPort: 7000, Ports: []int32{7000}}
		}), []any{8123, 9000, 9009}},
		{"worker", func() *appv1alpha1.App { a := sampleApp("w"); a.Spec.Type = appv1alpha1.TypeBackgroundWorker; return a }(), nil},
		{"cron", func() *appv1alpha1.App {
			a := sampleApp("c")
			a.Spec.Type, a.Spec.Schedule = appv1alpha1.TypeCronJob, "* * * * *"
			return a
		}(), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, _ := newService(nil, tc.app)
			view := service.view(tc.app)
			details := renderServiceDetails(view, effectiveType(tc.app.Spec.Type), "")
			got, present := details["openPorts"]
			if tc.want == nil {
				if present || view.OpenPorts != nil {
					t.Fatalf("openPorts on a type without a network address: %v / %v", got, view.OpenPorts)
				}
				return
			}
			list, ok := got.([]any)
			if !present || !ok || len(list) != len(tc.want) || len(view.OpenPorts) != len(tc.want) {
				t.Fatalf("openPorts = %#v (view %v), want ports %v", got, view.OpenPorts, tc.want)
			}
			for i, raw := range list {
				entry := raw.(map[string]any)
				for _, key := range required {
					if _, ok := entry[key]; !ok {
						t.Fatalf("entry %v lacks Render's required %q", entry, key)
					}
				}
				if entry["port"] != tc.want[i] || !slices.Contains(protocols, entry["protocol"].(string)) {
					t.Fatalf("entry %v, want port %v with a Render protocol %v", entry, tc.want[i], protocols)
				}
				if view.OpenPorts[i].Port != int32(tc.want[i].(int)) {
					t.Fatalf("GraphQL view port %d disagrees with REST %v", view.OpenPorts[i].Port, tc.want[i])
				}
			}
			// The published number and the address name the same primary.
			if len(tc.want) > 0 && view.Port != int32(tc.want[0].(int)) && tc.app.Spec.UsesImagePorts() {
				t.Fatalf("port %d is not the serving primary %v", view.Port, tc.want[0])
			}
			// ADR041's address follows the same primary on every surface, and a
			// pending image-port service has none rather than a guessed :3000.
			if tc.app.Spec.UsesImagePorts() {
				want := ""
				if len(tc.want) > 0 { // want[0] is the primary
					want = tc.app.Spec.PlatformSubdomain(tc.app.Name) + ":" + strconv.Itoa(tc.want[0].(int))
				}
				got, _ := details["internalAddress"].(string)
				if got != want || view.InternalAddress != want {
					t.Fatalf("internalAddress REST %q / GraphQL %q, want %q", got, view.InternalAddress, want)
				}
			}
		})
	}
}

// GraphQL keeps null (no network address) distinct from [] (image-port
// discovery pending): graphql-go renders a typed nil slice as [], so the
// resolver must hand it an untyped nil (w6/m148).
func TestGraphQLOpenPortsNullVersusPending(t *testing.T) {
	worker := sampleApp("worker")
	worker.Spec.Type = appv1alpha1.TypeBackgroundWorker
	pending := sampleApp("ch")
	pending.Spec.Type, pending.Spec.PortMode = appv1alpha1.TypePrivateService, appv1alpha1.PortModeImageV1
	serving := sampleApp("multi")
	serving.Spec.Type, serving.Spec.PortMode = appv1alpha1.TypePrivateService, appv1alpha1.PortModeImageV1
	serving.Status.ServingNetwork = &appv1alpha1.ImageNetworkStatus{Image: "i", Revision: "r", PrimaryPort: 8123, Ports: []int32{8123, 9000}}
	svc, _ := newService(nil, worker, pending, serving)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"worker": `null`,
		"ch":     `[]`,
		"multi":  `[{"port":8123,"protocol":"TCP"},{"port":9000,"protocol":"TCP"}]`,
	} {
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{ service(id: "` + name + `") { openPorts { port protocol } } }`})
		if len(res.Errors) > 0 {
			t.Fatalf("%s: %v", name, res.Errors)
		}
		got, _ := json.Marshal(res.Data.(map[string]any)["service"].(map[string]any)["openPorts"])
		if string(got) != want {
			t.Errorf("%s: openPorts = %s, want %s", name, got, want)
		}
	}
}

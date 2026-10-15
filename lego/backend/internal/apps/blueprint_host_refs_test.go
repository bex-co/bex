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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// hostRefLocation is one validation entry's display path and authored
// line/column.
type hostRefLocation struct {
	path         any
	line, column any
}

// hostRefLocations are the validation entries' locations, in returned order.
func hostRefLocations(v BlueprintValidation) []hostRefLocation {
	got := make([]hostRefLocation, 0, len(v.Errors))
	for _, e := range v.Errors {
		got = append(got, hostRefLocation{ptrValue(e.Path), ptrValue(e.Line), ptrValue(e.Column)})
	}
	return got
}

// TestValidateBlueprintLocatesEveryMissingHostReference (w8/m74, sweep 314):
// a missing fromService host target is located at its own authored envVars
// entry. The pointer used to be recovered by matching quoted names in the
// message, so a valid self-reference, an earlier entry naming the same target,
// or another service's valid reference to the consumer took the error.
func TestValidateBlueprintLocatesEveryMissingHostReference(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest string
		want     []hostRefLocation
	}{
		{
			name: "self-only",
			manifest: `services:
- type: web
  name: qa314-ebe6d7-consumer
  runtime: image
  image:
    url: nginx:1.27
  plan: free
  envVars:
  - key: OWN_HOST
    fromService:
      type: web
      name: qa314-ebe6d7-consumer
      property: host
`,
			want: []hostRefLocation{},
		},
		{
			name: "missing-only",
			manifest: `services:
- type: web
  name: qa314-ebe6d7-consumer
  runtime: image
  image:
    url: nginx:1.27
  plan: free
  envVars:
  - key: MISSING_HOST
    fromService:
      type: web
      name: qa314-ebe6d7-missing
      property: host
`,
			want: []hostRefLocation{{"services[0].envVars[0]", 9, 5}},
		},
		{
			name: "self-before-missing",
			manifest: `services:
- type: web
  name: qa314-ebe6d7-consumer
  runtime: image
  image:
    url: nginx:1.27
  plan: free
  envVars:
  - key: OWN_HOST
    fromService:
      type: web
      name: qa314-ebe6d7-consumer
      property: host
  - key: MISSING_HOST
    fromService:
      type: web
      name: qa314-ebe6d7-missing
      property: host
`,
			want: []hostRefLocation{{"services[0].envVars[1]", 14, 5}},
		},
		{
			name: "missing-before-self",
			manifest: `services:
- type: web
  name: qa314-ebe6d7-consumer
  runtime: image
  image:
    url: nginx:1.27
  plan: free
  envVars:
  - key: MISSING_HOST
    fromService:
      type: web
      name: qa314-ebe6d7-missing
      property: host
  - key: OWN_HOST
    fromService:
      type: web
      name: qa314-ebe6d7-consumer
      property: host
`,
			want: []hostRefLocation{{"services[0].envVars[0]", 9, 5}},
		},
		{
			name: "two-missing-same-target",
			manifest: `services:
- type: web
  name: qa314-ebe6d7-consumer
  runtime: image
  image:
    url: nginx:1.27
  plan: free
  envVars:
  - key: MISSING_ONE
    fromService:
      type: web
      name: qa314-ebe6d7-missing
      property: host
  - key: MISSING_TWO
    fromService:
      type: web
      name: qa314-ebe6d7-missing
      property: host
`,
			want: []hostRefLocation{{"services[0].envVars[0]", 9, 5}, {"services[0].envVars[1]", 14, 5}},
		},
		{
			name: "project-self-before-missing",
			manifest: `projects:
- name: qa314-ebe6d7-project
  environments:
  - name: production
    services:
    - type: web
      name: qa314-ebe6d7-consumer
      runtime: image
      image:
        url: nginx:1.27
      plan: free
      envVars:
      - key: OWN_HOST
        fromService:
          type: web
          name: qa314-ebe6d7-consumer
          property: host
      - key: MISSING_HOST
        fromService:
          type: web
          name: qa314-ebe6d7-missing
          property: host
`,
			want: []hostRefLocation{{"projects[0].environments[0].services[0].envVars[1]", 18, 9}},
		},
		{
			name: "project-two-missing-same-target",
			manifest: `projects:
- name: qa314-ebe6d7-project
  environments:
  - name: production
    services:
    - type: web
      name: qa314-ebe6d7-consumer
      runtime: image
      image:
        url: nginx:1.27
      plan: free
      envVars:
      - key: MISSING_ONE
        fromService:
          type: web
          name: qa314-ebe6d7-missing
          property: host
      - key: MISSING_TWO
        fromService:
          type: web
          name: qa314-ebe6d7-missing
          property: host
`,
			want: []hostRefLocation{{"projects[0].environments[0].services[0].envVars[0]", 13, 9}, {"projects[0].environments[0].services[0].envVars[1]", 18, 9}},
		},
		{
			name: "cross-consumer-valid-reference-before-missing",
			manifest: `services:
- type: web
  name: qa314-ebe6d7-consumer
  runtime: image
  image:
    url: nginx:1.27
  plan: free
  envVars:
  - key: OWN_HOST
    fromService:
      type: web
      name: qa314-ebe6d7-consumer-two
      property: host
- type: web
  name: qa314-ebe6d7-consumer-two
  runtime: image
  image:
    url: nginx:1.27
  plan: free
  envVars:
  - key: MISSING_HOST
    fromService:
      type: web
      name: qa314-ebe6d7-missing
      property: host
`,
			want: []hostRefLocation{{"services[1].envVars[0]", 21, 5}},
		},
		{
			name: "cross-consumer-two-missing-same-target",
			manifest: `services:
- type: web
  name: qa314-ebe6d7-consumer
  runtime: image
  image:
    url: nginx:1.27
  plan: free
  envVars:
  - key: MISSING_HOST
    fromService:
      type: web
      name: qa314-ebe6d7-missing
      property: host
- type: web
  name: qa314-ebe6d7-consumer-two
  runtime: image
  image:
    url: nginx:1.27
  plan: free
  envVars:
  - key: MISSING_HOST
    fromService:
      type: web
      name: qa314-ebe6d7-missing
      property: host
`,
			want: []hostRefLocation{{"services[0].envVars[0]", 9, 5}, {"services[1].envVars[0]", 21, 5}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newBlueprintValidator(t)
			svc.Workspace = fakeWorkspace{"id-a": connOwner}
			v, err := svc.ValidateBlueprint(ownershipCtx(), connOwner, tc.manifest, "")
			if err != nil {
				t.Fatal(err)
			}
			if v.Valid != (len(tc.want) == 0) {
				t.Fatalf("valid = %v, errors %+v", v.Valid, v.Errors)
			}
			if got := hostRefLocations(v); fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("locations = %v, want %v", got, tc.want)
			}
			for _, e := range v.Errors {
				if !strings.Contains(e.Error, `fromService references unknown service "qa314-ebe6d7-missing"`) {
					t.Errorf("message changed: %q", e.Error)
				}
			}
		})
	}
}

// Repeated keys and earlier literals do not move a missing host's location:
// the authored ordinal locates it, not the first entry sharing its key.
func TestValidateBlueprintLocatesRepeatedKeyMissingHost(t *testing.T) {
	const manifest = `services:
  - type: web
    name: consumer
    runtime: image
    image: {url: nginx:1.27}
    envVars:
      - {key: UPSTREAM, value: literal}
      - {key: OTHER, value: x}
      - {key: UPSTREAM, fromService: {type: web, name: absent, property: host}}
      - {key: UPSTREAM, fromService: {type: web, name: absent, property: host}}
`
	svc := newBlueprintValidator(t)
	svc.Workspace = fakeWorkspace{"id-a": connOwner}
	v, err := svc.ValidateBlueprint(ownershipCtx(), connOwner, manifest, "")
	if err != nil {
		t.Fatal(err)
	}
	want := []hostRefLocation{{"services[0].envVars[2]", 9, 9}, {"services[0].envVars[3]", 10, 9}}
	if got := hostRefLocations(v); v.Valid || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("locations = %v (valid %v), want %v", got, v.Valid, want)
	}
}

// existingHostTargets are one workspace App per actual service kind, plus one
// whose stored type is empty (the web_service default).
var existingHostTargets = map[string]string{
	"ex-web":     appv1alpha1.TypeWebService,
	"ex-pserv":   appv1alpha1.TypePrivateService,
	"ex-worker":  appv1alpha1.TypeBackgroundWorker,
	"ex-cron":    appv1alpha1.TypeCronJob,
	"ex-static":  appv1alpha1.TypeStaticSite,
	"ex-default": "",
}

// existingHostService is a validator whose workspace already holds every
// existingHostTargets App.
func existingHostService(t *testing.T) *Service {
	t.Helper()
	svc := newBlueprintValidator(t)
	svc.Workspace = fakeWorkspace{"id-a": connOwner}
	for name, appType := range existingHostTargets {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{core.LabelTenant: connOwner, core.LabelServiceName: name}},
			Spec:       appv1alpha1.AppSpec{Type: appType, Image: "nginx:1.27", Port: 3000, Subdomain: name + "-slug"},
		}
		if err := svc.Client.Create(ownershipCtx(), app); err != nil {
			t.Fatal(err)
		}
	}
	return svc
}

// existingHostManifest is one consumer referencing target's host with refType,
// at the root or nested in a project environment, plus extra declarations.
func existingHostManifest(nested bool, refType, target, extra string) string {
	services := fmt.Sprintf(`- type: web
  name: consumer
  runtime: image
  image: {url: nginx:1.27}
  envVars:
    - key: UPSTREAM
      fromService:
        type: %s
        name: %s
        property: host
%s`, refType, target, extra)
	if !nested {
		return "services:\n" + services
	}
	indented := strings.ReplaceAll(strings.TrimRight(services, "\n"), "\n", "\n        ")
	return "projects:\n  - name: proj\n    environments:\n      - name: production\n        services:\n        " + indented + "\n"
}

// TestValidateBlueprintChecksExistingHostTargets (w8/m74, sweep 316): a host
// reference to an App that already exists in the workspace is held to the
// same contract as a declared sibling (w8/081, w8/096) — its supplied type
// must name the target's actual kind, and the target must have a network
// address (web/private only). Both used to validate clean.
func TestValidateBlueprintChecksExistingHostTargets(t *testing.T) {
	const (
		mismatch  = "does not match"
		noAddress = "has no network address"
		unknown   = "fromService references unknown service"
	)
	for _, tc := range []struct {
		target, refType string
		extra           string // further declarations
		want            string // "" = valid; else the refusal's message fragment
		field           string // the refusal's path under envVars[0]
	}{
		{target: "ex-web", refType: "web"},
		{target: "ex-pserv", refType: "pserv"},
		{target: "ex-default", refType: "web"},
		{target: "ex-web", refType: "worker", want: mismatch, field: ".fromService.type"},
		{target: "ex-web", refType: "pserv", want: mismatch, field: ".fromService.type"},
		{target: "ex-pserv", refType: "web", want: mismatch, field: ".fromService.type"},
		{target: "ex-default", refType: "worker", want: mismatch, field: ".fromService.type"},
		{target: "ex-worker", refType: "web", want: mismatch, field: ".fromService.type"},
		{target: "ex-worker", refType: "worker", want: noAddress},
		{target: "ex-cron", refType: "cron", want: noAddress},
		{target: "ex-static", refType: "web", want: noAddress},
		{target: "ex-static", refType: "static", want: noAddress},
		{target: "ex-missing", refType: "web", want: unknown},
		// A declared sibling takes precedence over the same-named existing App,
		// whichever way its own kind decides.
		{target: "ex-static", refType: "web", extra: "- {type: web, name: ex-static, runtime: image, image: {url: nginx:1.27}}\n"},
		{target: "ex-web", refType: "worker", extra: "- {type: worker, name: ex-web, runtime: image, image: {url: nginx:1.27}}\n", want: noAddress},
	} {
		for _, nested := range []bool{false, true} {
			name := fmt.Sprintf("%s/%s/nested=%v/declared=%v", tc.target, tc.refType, nested, tc.extra != "")
			t.Run(name, func(t *testing.T) {
				svc := existingHostService(t)
				v, err := svc.ValidateBlueprint(ownershipCtx(), connOwner, existingHostManifest(nested, tc.refType, tc.target, tc.extra), "")
				if err != nil {
					t.Fatal(err)
				}
				if tc.want == "" {
					if !v.Valid {
						t.Fatalf("errors = %+v, want valid", v.Errors)
					}
					return
				}
				prefix := "services[0]"
				if nested {
					prefix = "projects[0].environments[0].services[0]"
				}
				if v.Valid || len(v.Errors) != 1 {
					t.Fatalf("validation = %+v, want one refusal", v)
				}
				e := v.Errors[0]
				if got, want := fmt.Sprint(ptrValue(e.Path)), prefix+".envVars[0]"+tc.field; got != want || !strings.Contains(e.Error, tc.want) || e.Line == nil || e.Column == nil {
					t.Fatalf("refusal %q at %s line %v column %v, want %q at %s", e.Error, got, ptrValue(e.Line), ptrValue(e.Column), tc.want, want)
				}
				if tc.want == mismatch && !strings.Contains(e.Error, fmt.Sprintf(`fromService.type %q does not match %q`, tc.refType, tc.target)) {
					t.Fatalf("mismatch %q does not name the supplied type and target", e.Error)
				}
			})
		}
	}
}

// TestDeployStackRefusesInvalidExistingHostTargetBeforeWriting: the apply
// shares validation's existing-target checks and refuses before any write,
// with the same message; the correctly typed existing web control applies
// and resolves the target's slug.
func TestDeployStackRefusesInvalidExistingHostTargetBeforeWriting(t *testing.T) {
	for _, tc := range []struct {
		name, refType, target, want string
	}{
		{"static-host", "web", "ex-static", `envVars["UPSTREAM"] fromService host references "ex-static" which has no network address`},
		{"web-wrong-type", "worker", "ex-web", `envVars["UPSTREAM"] fromService.type "worker" does not match "ex-web" (want type web)`},
		{"missing", "web", "ex-missing", `fromService references unknown service "ex-missing"`},
	} {
		for _, nested := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/nested=%v", tc.name, nested), func(t *testing.T) {
				svc := existingHostService(t)
				manifest := existingHostManifest(nested, tc.refType, tc.target, "")
				_, err := svc.DeployStack(ownershipCtx(), DeployRequest{OwnerID: connOwner, Manifest: manifest})
				if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("DeployStack = %v, want a 400 containing %q", err, tc.want)
				}
				v, verr := svc.ValidateBlueprint(ownershipCtx(), connOwner, manifest, "")
				if verr != nil || len(v.Errors) != 1 || !strings.HasSuffix(err.Error(), v.Errors[0].Error) {
					t.Fatalf("validate %+v (err %v) disagrees with apply %v", v.Errors, verr, err)
				}
				var apps appv1alpha1.AppList
				if err := svc.Client.List(ownershipCtx(), &apps); err != nil {
					t.Fatal(err)
				}
				if len(apps.Items) != len(existingHostTargets) {
					t.Fatalf("refused apply left %d Apps, want only the %d existing", len(apps.Items), len(existingHostTargets))
				}
			})
		}
	}
	t.Run("existing-web-control", func(t *testing.T) {
		svc := existingHostService(t)
		if _, err := svc.DeployStack(ownershipCtx(), DeployRequest{OwnerID: connOwner, Manifest: existingHostManifest(false, "web", "ex-web", "")}); err != nil {
			t.Fatalf("DeployStack: %v", err)
		}
		var apps appv1alpha1.AppList
		if err := svc.Client.List(ownershipCtx(), &apps); err != nil {
			t.Fatal(err)
		}
		for i := range apps.Items {
			if core.AppPublicName(&apps.Items[i]) != "consumer" {
				continue
			}
			for _, env := range apps.Items[i].Spec.Env {
				if env.Name == "UPSTREAM" && env.Value == "ex-web-slug" {
					return
				}
			}
			t.Fatalf("consumer env = %+v, want UPSTREAM=ex-web-slug", apps.Items[i].Spec.Env)
		}
		t.Fatal("consumer was not created")
	})
}

// TestHostReferenceRefusalsAgreeAcrossValidationSurfaces: the CLI's REST
// multipart validation, the dashboard's GraphQL validateBlueprint (legacy
// errors and errorDetails) and MCP validate_bex_yml return the same located
// host-reference refusals as the shared core, with REST answering 200.
func TestHostReferenceRefusalsAgreeAcrossValidationSurfaces(t *testing.T) {
	svc := existingHostService(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	schema := blueprintSchema(t, svc)
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ownershipCtx(), serverT, nil); err != nil {
		t.Fatal(err)
	}
	mc, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(context.Background(), clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mc.Close() })

	type detail struct {
		Error  string `json:"error"`
		Path   string `json:"path"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	}
	for name, doc := range map[string]string{
		"missing twice": `services:
  - type: web
    name: consumer
    runtime: image
    image: {url: nginx:1.27}
    envVars:
      - {key: ONE, fromService: {type: web, name: absent, property: host}}
      - {key: TWO, fromService: {type: web, name: absent, property: host}}
`,
		"existing static and wrong type": `services:
  - type: web
    name: consumer
    runtime: image
    image: {url: nginx:1.27}
    envVars:
      - {key: SITE, fromService: {type: web, name: ex-static, property: host}}
      - {key: API, fromService: {type: worker, name: ex-web, property: host}}
`,
	} {
		t.Run(name, func(t *testing.T) {
			direct, err := svc.ValidateBlueprint(ownershipCtx(), connOwner, doc, "")
			if err != nil || direct.Valid || len(direct.Errors) != 2 {
				t.Fatalf("direct = %+v, %v; want two refusals", direct, err)
			}
			want := make([]detail, 0, len(direct.Errors))
			for _, e := range direct.Errors {
				want = append(want, detail{e.Error, *e.Path, *e.Line, *e.Column})
			}

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, multipartBlueprintRequest(t, connOwner, "render.yaml", doc).WithContext(ownershipCtx()))
			var rest struct {
				Valid  bool     `json:"valid"`
				Errors []detail `json:"errors"`
			}
			if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &rest) != nil || rest.Valid || fmt.Sprint(rest.Errors) != fmt.Sprint(want) {
				t.Fatalf("REST %d %s, want %+v", rec.Code, rec.Body, want)
			}

			res := graphql.Do(graphql.Params{Schema: schema, Context: ownershipCtx(), RequestString: `query($y: String!, $o: String) { validateBlueprint(bexYaml: $y, ownerId: $o) { valid errors errorDetails { error path line column } } }`,
				VariableValues: map[string]any{"y": doc, "o": connOwner}})
			if len(res.Errors) > 0 {
				t.Fatalf("GraphQL: %v", res.Errors)
			}
			raw, _ := json.Marshal(res.Data.(map[string]any)["validateBlueprint"])
			var gql struct {
				Valid        bool     `json:"valid"`
				Errors       []string `json:"errors"`
				ErrorDetails []detail `json:"errorDetails"`
			}
			if err := json.Unmarshal(raw, &gql); err != nil || gql.Valid || fmt.Sprint(gql.ErrorDetails) != fmt.Sprint(want) || len(gql.Errors) != len(want) || gql.Errors[1] != want[1].Error {
				t.Fatalf("GraphQL = %s, want %+v", raw, want)
			}

			call, err := mc.CallTool(context.Background(), &mcp.CallToolParams{Name: "validate_bex_yml", Arguments: map[string]any{"bexYaml": doc}})
			if err != nil || call.IsError {
				t.Fatalf("MCP: %v %+v", err, call)
			}
			raw, _ = json.Marshal(call.StructuredContent)
			rest.Errors = nil
			if err := json.Unmarshal(raw, &rest); err != nil || rest.Valid || fmt.Sprint(rest.Errors) != fmt.Sprint(want) {
				t.Fatalf("MCP = %s, want %+v", raw, want)
			}
		})
	}
}

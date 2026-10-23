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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// predeploy_test.go covers App.spec.preDeployCommand (Render's Pre-Deploy
// Command, w1/m33) threaded + readable identically across bex.yml, REST,
// GraphQL, and MCP, plus the SetPreDeployCommand update verb — the same
// field-threading depth rootdir_test.go applies to spec.rootDir.

func TestRESTCreateThreadsPreDeployCommandAndReadsItBack(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","repo":"https://github.com/x/mono","preDeployCommand":"npm run migrate"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "npm run migrate" {
		t.Fatalf("spec.preDeployCommand = %q, want npm run migrate", got)
	}

	// Read-back: Render nests it under serviceDetails.preDeployCommand.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
	var out struct {
		ServiceDetails struct {
			PreDeployCommand string `json:"preDeployCommand"`
		} `json:"serviceDetails"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ServiceDetails.PreDeployCommand != "npm run migrate" {
		t.Errorf("GET response = %s, want serviceDetails.preDeployCommand npm run migrate", rec.Body)
	}
}

func TestRESTCreateAcceptsPreDeployUnderServiceDetails(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// Render-faithful nesting; top-level absent, so the nested value wins.
	body := `{"name":"web","repo":"https://github.com/x/mono","serviceDetails":{"preDeployCommand":"rails db:migrate"}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "rails db:migrate" {
		t.Errorf("serviceDetails.preDeployCommand not threaded: spec.preDeployCommand = %q", got)
	}
}

func TestGraphQLCreateAndReadPreDeployCommand(t *testing.T) {
	svc, cl := newService(nil)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { createService(name: "web", repo: "https://github.com/x/mono", preDeployCommand: "npm run migrate") { preDeployCommand } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("createService: %v", res.Errors)
	}
	if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "npm run migrate" {
		t.Errorf("createService did not thread preDeployCommand onto the spec: got %q", got)
	}

	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ server(id: "web") { preDeployCommand } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("server query: %v", res.Errors)
	}
	srv := res.Data.(map[string]any)["server"].(map[string]any)
	if srv["preDeployCommand"] != "npm run migrate" {
		t.Errorf("server.preDeployCommand = %+v, want npm run migrate", srv["preDeployCommand"])
	}
}

func TestMCPCreateWebServiceThreadsPreDeployCommand(t *testing.T) {
	req := createWebServiceArgs{Name: "w", Repo: "https://github.com/x/mono", PreDeployCommand: "npm run migrate"}.toCreateRequest()
	if req.PreDeployCommand != "npm run migrate" {
		t.Errorf("create_web_service preDeployCommand not threaded: %+v", req)
	}
}

func TestDeployManifestThreadsPreDeployCommand(t *testing.T) {
	svc, cl := newService(nil)
	manifest := "services:\n  - name: hello\n    type: web\n    runtime: docker\n    repo: https://github.com/x/mono\n    preDeployCommand: npm run migrate\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if got := getApp(t, cl, "hello").Spec.PreDeployCommand; got != "npm run migrate" {
		t.Errorf("bex.yml preDeployCommand not threaded: spec.preDeployCommand = %q", got)
	}
}

// paidRepoApp is a Git-backed web service on a paid plan: a pre-deploy command
// is a paid-plan feature (w8/m131).
func paidRepoApp(name string) *appv1alpha1.App {
	a := repoApp(name, "https://github.com/x/mono", "main")
	a.Spec.Tier = "starter"
	return a
}

// --- SetPreDeployCommand (update-after-create) ---

func TestSetPreDeployCommandSetsAndClears(t *testing.T) {
	svc, cl := newService(nil, paidRepoApp("web"))

	v, err := svc.SetPreDeployCommand(context.Background(), "web", "  npm run migrate  ")
	if err != nil {
		t.Fatalf("SetPreDeployCommand: %v", err)
	}
	// Nonblank command text is stored verbatim, padding included (w8/m68).
	if v.PreDeployCommand != "  npm run migrate  " {
		t.Errorf("view PreDeployCommand = %q, want the exact command", v.PreDeployCommand)
	}
	if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "  npm run migrate  " {
		t.Errorf("spec.preDeployCommand = %q, want the exact command", got)
	}

	// Empty and whitespace-only clear the step.
	for _, blank := range []string{"", " \t "} {
		if _, err := svc.SetPreDeployCommand(context.Background(), "web", "npm run migrate"); err != nil {
			t.Fatalf("SetPreDeployCommand: %v", err)
		}
		if _, err := svc.SetPreDeployCommand(context.Background(), "web", blank); err != nil {
			t.Fatalf("SetPreDeployCommand(%q): %v", blank, err)
		}
		if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "" {
			t.Errorf("SetPreDeployCommand(%q): spec.preDeployCommand = %q, want empty (cleared)", blank, got)
		}
	}
}

func TestSetPreDeployCommandRejectsCronAndStatic(t *testing.T) {
	svc, cl := newService(nil, cronApp("nightly"))
	if _, err := svc.SetPreDeployCommand(context.Background(), "nightly", "npm run migrate"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("SetPreDeployCommand on a cron_job should be core.ErrBadRequest, got %v", err)
	}
	if got := getApp(t, cl, "nightly").Spec.PreDeployCommand; got != "" {
		t.Errorf("a rejected SetPreDeployCommand must not touch the spec, got %q", got)
	}
}

func TestRESTPatchServicePreDeployCommand(t *testing.T) {
	svc, cl := newService(nil, paidRepoApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", strings.NewReader(`{"preDeployCommand":"npm run migrate"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH preDeployCommand => 200, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "npm run migrate" {
		t.Errorf("PATCH did not set spec.preDeployCommand, got %q", got)
	}
}

// A pre-deploy command is a paid-plan feature, as on Render (w8/m131): every
// writer refuses a command on the free tier with one message, clearing is
// allowed on any plan, a paid service must clear the command before moving to
// free (in the same PATCH too), and a legacy free service that already carries
// a command keeps accepting unrelated writes.
func TestPreDeployCommandIsAPaidPlanFeature(t *testing.T) {
	const refused = "pre-deploy command is available only for paid web services, private services, and background workers"
	ctx := context.Background()
	freeApp := func() *appv1alpha1.App {
		a := repoApp("web", "https://github.com/x/mono", "main")
		a.Spec.Tier, a.Spec.Replicas = "free", 1
		return a
	}

	t.Run("create on free is refused and creates nothing", func(t *testing.T) {
		svc, cl := newService(nil)
		_, err := svc.Create(ctx, CreateRequest{Name: "web", Type: appv1alpha1.TypeWebService, Repo: "https://github.com/x/mono", Plan: "free", PreDeployCommand: "echo x"})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), refused) {
			t.Fatalf("Create = %v, want a 400 naming the paid-plan rule", err)
		}
		var apps appv1alpha1.AppList
		if err := cl.List(ctx, &apps); err != nil || len(apps.Items) != 0 {
			t.Fatalf("refused create left %d Apps (%v)", len(apps.Items), err)
		}
		if _, err := svc.Create(ctx, CreateRequest{Name: "web", Type: appv1alpha1.TypeWebService, Repo: "https://github.com/x/mono", Plan: "starter", PreDeployCommand: "echo x"}); err != nil {
			t.Fatalf("Create on starter: %v", err)
		}
	})

	t.Run("update on free is refused, clearing is allowed", func(t *testing.T) {
		svc, cl := newService(nil, freeApp())
		if _, err := svc.SetPreDeployCommand(ctx, "web", "echo x"); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), refused) {
			t.Fatalf("SetPreDeployCommand = %v, want the paid-plan refusal", err)
		}
		if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "" {
			t.Fatalf("refused write stored %q", got)
		}
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{PreDeployCommand: new("echo x")}); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("ApplyServicePatch on free = %v, want a 400", err)
		}
	})

	t.Run("legacy free command: unrelated write succeeds, clearing succeeds", func(t *testing.T) {
		a := freeApp()
		a.Spec.PreDeployCommand = "echo legacy"
		svc, cl := newService(nil, a)
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{Plan: new("free"), HealthCheckPath: new("/up")}); err != nil {
			t.Fatalf("unrelated write on a legacy free service: %v", err)
		}
		if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "echo legacy" {
			t.Fatalf("command = %q, want the legacy command kept", got)
		}
		if _, err := svc.SetPreDeployCommand(ctx, "web", ""); err != nil {
			t.Fatalf("clearing on free: %v", err)
		}
	})

	t.Run("downgrade needs the command cleared first", func(t *testing.T) {
		a := paidRepoApp("web")
		a.Spec.Replicas = 1
		a.Spec.PreDeployCommand = "echo x"
		svc, cl := newService(nil, a)
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{Plan: new("free")}); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "clear the pre-deploy command before changing to the free plan") {
			t.Fatalf("downgrade with a command = %v, want the clear-first refusal", err)
		}
		if got := getApp(t, cl, "web").Spec.Tier; got != "starter" {
			t.Fatalf("refused downgrade moved the tier to %q", got)
		}
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{Plan: new("free"), PreDeployCommand: new(""), PreDeployBeforeFreeDowngrade: true}); err != nil {
			t.Fatalf("clear + downgrade in one PATCH: %v", err)
		}
		got := getApp(t, cl, "web").Spec
		if got.Tier != "free" || got.PreDeployCommand != "" {
			t.Fatalf("after clear + downgrade: tier %q command %q", got.Tier, got.PreDeployCommand)
		}
	})

	t.Run("set + upgrade in one PATCH validates against the final plan", func(t *testing.T) {
		svc, cl := newService(nil, freeApp())
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{Plan: new("starter"), PreDeployCommand: new("echo x"), PreDeployBeforeFreeDowngrade: true}); err != nil {
			t.Fatalf("upgrade + set: %v", err)
		}
		if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "echo x" {
			t.Fatalf("command = %q", got)
		}
	})

	t.Run("blueprint refuses it at the field and accepts it on starter", func(t *testing.T) {
		svc := workspaceValidator(t)
		doc := func(plan string) string {
			return "services:\n  - {type: web, name: a, runtime: image, image: {url: nginx:1.27}, plan: " + plan + ", preDeployCommand: echo migrate}\n"
		}
		v, err := svc.ValidateBlueprint(ownershipCtx(), connOwner, doc("free"), "")
		if err != nil || v.Valid || len(v.Errors) != 1 || v.Errors[0].Path == nil || *v.Errors[0].Path != "services[0].preDeployCommand" || !strings.Contains(v.Errors[0].Error, refused) {
			t.Fatalf("free = %+v (%v), want one refusal at services[0].preDeployCommand", v, err)
		}
		if v, err := svc.ValidateBlueprint(ownershipCtx(), connOwner, doc("starter"), ""); err != nil || !v.Valid {
			t.Fatalf("starter = %+v (%v), want valid", v, err)
		}
	})
}

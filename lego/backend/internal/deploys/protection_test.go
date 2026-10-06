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

package deploys

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeProtection stands in for apps' guard: protected names need the exact
// "sudo <verb> service <name>" phrase on the context.
type fakeProtection struct {
	protected map[string]bool
	err       error
	verbs     []string
}

func (f *fakeProtection) AppProtected(_ context.Context, a *appv1alpha1.App) (bool, error) {
	return f.protected[a.Name], f.err
}

func (f *fakeProtection) RequireUnprotected(ctx context.Context, a *appv1alpha1.App, verb string) error {
	f.verbs = append(f.verbs, verb)
	protected, err := f.AppProtected(ctx, a)
	if err != nil {
		return err
	}
	if want := "sudo " + verb + " service " + a.Name; protected && core.ConfirmFrom(ctx) != want {
		return fmt.Errorf("%w: %q is a member of a protected environment; retry with confirm=%q to %s it", core.ErrBadRequest, a.Name, want, verb)
	}
	return nil
}

const repointWeb = "sudo repoint service web"

// TestOverrideDeployNeedsRepointOnProtectedMember is w4/m176: imageUrl and
// commitId select the code a deploy runs, so on a protected member they need
// the image setter's "repoint" phrase. A bare redeploy stays exempt.
func TestOverrideDeployNeedsRepointOnProtectedMember(t *testing.T) {
	newProtected := func() (*Service, *fakeStore, *fakeProtection) {
		ds := newFakeStore()
		app := sampleApp("web", "srv-1")
		app.Spec.Image = "ghcr.io/acme/web:v1"
		svc, _ := newService(ds, app)
		guard := &fakeProtection{protected: map[string]bool{"web": true}}
		svc.Protection = guard
		return svc, ds, guard
	}
	override := TriggerParams{ImageURL: "ghcr.io/acme/web:v2"}

	svc, ds, guard := newProtected()
	_, err := svc.Trigger(context.Background(), "web", override)
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), repointWeb) {
		t.Fatalf("unconfirmed override = %v, want the refusal naming %q", err, repointWeb)
	}
	if ds.nextID != 0 {
		t.Fatalf("a refused override opened %d deploy rows", ds.nextID)
	}
	if strings.Join(guard.verbs, ",") != "repoint" {
		t.Fatalf("guard asked for %v, want the repoint class", guard.verbs)
	}
	if _, err := svc.Trigger(core.WithConfirm(context.Background(), "sudo deploy service web"), "web", override); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("wrong phrase = %v, want the same refusal", err)
	}

	if d, err := svc.Trigger(core.WithConfirm(context.Background(), repointWeb), "web", override); err != nil || d.Image != override.ImageURL {
		t.Fatalf("confirmed override = %+v, %v", d, err)
	}

	svc, ds, guard = newProtected()
	if _, err := svc.Trigger(context.Background(), "web", TriggerParams{}); err != nil || ds.nextID != 1 {
		t.Fatalf("bare redeploy on a protected member = %v (rows %d), want it unguarded", err, ds.nextID)
	}
	if len(guard.verbs) != 0 {
		t.Fatalf("a bare redeploy consulted the guard: %v", guard.verbs)
	}

	// A protection lookup that fails refuses the override (fail closed).
	svc, _, guard = newProtected()
	guard.err = errors.New("store down")
	if _, err := svc.Trigger(context.Background(), "web", override); err == nil {
		t.Fatal("override with an unreadable protection status succeeded")
	}
}

func TestRollbackNeedsRepointOnProtectedMember(t *testing.T) {
	setup := func(protected bool) (*Service, string) {
		ds := newFakeStore()
		first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{}, "")
		if won, err := ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "web:v1"); err != nil || !won {
			t.Fatalf("close first: won=%v err=%v", won, err)
		}
		app := sampleApp("web", "srv-1")
		app.Spec.Image = "web:v2"
		svc, _ := newService(ds, app)
		svc.Protection = &fakeProtection{protected: map[string]bool{"web": protected}}
		return svc, first.ID
	}

	svc, target := setup(true)
	if _, err := svc.Rollback(context.Background(), "web", target); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), repointWeb) {
		t.Fatalf("unconfirmed rollback = %v, want the refusal naming %q", err, repointWeb)
	}
	if d, err := svc.Rollback(core.WithConfirm(context.Background(), repointWeb), "web", target); err != nil || d.Image != "web:v1" {
		t.Fatalf("confirmed rollback = %+v, %v", d, err)
	}

	svc, target = setup(false)
	if _, err := svc.Rollback(context.Background(), "web", target); err != nil {
		t.Fatalf("rollback on an unprotected service = %v, want no phrase needed", err)
	}
}

// The projection reports the phrase only once a rollback target exists, so a
// client is never asked to confirm a rollback the verb would refuse anyway.
func TestActionCapabilities_RollbackReportsProtectedConfirmation(t *testing.T) {
	ds := newFakeStore()
	svc, _ := newService(ds, sampleApp("web", "srv-1"))
	svc.Protection = &fakeProtection{protected: map[string]bool{"web": true}}
	ctx := context.Background()

	acts, err := svc.ActionCapabilities(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	if r := actionByID(t, acts, core.ActionRollback); r.Precondition != core.PrecondNoEligibleRollbackTarget {
		t.Fatalf("no target = %+v, want no_eligible_rollback_target", r)
	}
	first, _ := ds.CreateDeploy(ctx, "srv-1", "create", "web:v0", 1, store.CommitInfo{}, "")
	if won, err := ds.CloseDeploy(ctx, first.ID, store.DeployLive, "web:v0"); err != nil || !won {
		t.Fatalf("close: won=%v err=%v", won, err)
	}
	acts, err = svc.ActionCapabilities(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	if r := actionByID(t, acts, core.ActionRollback); r.Precondition != core.PrecondProtectedConfirmation {
		t.Fatalf("protected rollback = %+v, want protected_confirmation_required", r)
	}
	if d := actionByID(t, acts, core.ActionDeploy); d.Precondition != "" {
		t.Fatalf("bare deploy = %+v, want ready: it stays unguarded", d)
	}
}

// TestProtectedDeployVerbsAcrossTransports: REST ?confirm=, GraphQL confirm and
// MCP confirm all arm the same guard, for both verbs, and a missing phrase is
// refused on each with the phrase named.
func TestProtectedDeployVerbsAcrossTransports(t *testing.T) {
	type call func(t *testing.T, svc *Service, verb, target, confirm string) (ok bool, message string)
	rest := func(t *testing.T, svc *Service, verb, target, confirm string) (bool, string) {
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		path, body := "/v1/services/web/deploys", `{"imageUrl":"web:v3"}`
		if verb == "rollback" {
			path, body = "/v1/services/web/rollback", `{"deployId":"`+target+`"}`
		}
		if confirm != "" {
			path += "?confirm=" + url.QueryEscape(confirm)
		}
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code == http.StatusCreated, rec.Body.String()
	}
	gql := func(t *testing.T, svc *Service, verb, target, confirm string) (bool, string) {
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		q := `mutation($c: String) { triggerDeploy(serviceId: "web", imageUrl: "web:v3", confirm: $c) { id } }`
		if verb == "rollback" {
			q = `mutation($c: String) { rollbackService(serviceId: "web", deployId: "` + target + `", confirm: $c) { id } }`
		}
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: q, VariableValues: map[string]any{"c": confirm}})
		if len(res.Errors) > 0 {
			return false, res.Errors[0].Message
		}
		return true, ""
	}
	mcpCall := func(t *testing.T, svc *Service, verb, target, confirm string) (bool, string) {
		name, args := "trigger_deploy", map[string]any{"serviceId": "web", "imageUrl": "web:v3"}
		if verb == "rollback" {
			name, args = "rollback_deploy", map[string]any{"serviceId": "web", "deployId": target}
		}
		if confirm != "" {
			args["confirm"] = confirm
		}
		res, err := newMCPSession(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		var message string
		for _, content := range res.Content {
			if text, ok := content.(*mcp.TextContent); ok {
				message += text.Text
			}
		}
		return !res.IsError, message
	}

	for transport, do := range map[string]call{"REST": rest, "GraphQL": gql, "MCP": mcpCall} {
		for _, verb := range []string{"override deploy", "rollback"} {
			t.Run(transport+"/"+verb, func(t *testing.T) {
				ds := newFakeStore()
				first, _ := ds.CreateDeploy(context.Background(), "srv-1", "create", "web:v1", 1, store.CommitInfo{}, "")
				if won, err := ds.CloseDeploy(context.Background(), first.ID, store.DeployLive, "web:v1"); err != nil || !won {
					t.Fatalf("close first: won=%v err=%v", won, err)
				}
				app := sampleApp("web", "srv-1")
				app.Spec.Image = "web:v2"
				svc, _ := newService(ds, app)
				svc.Protection = &fakeProtection{protected: map[string]bool{"web": true}}

				if ok, message := do(t, svc, verb, first.ID, ""); ok || !strings.Contains(message, "protected environment") || !strings.Contains(message, "repoint service web") {
					t.Fatalf("unconfirmed %s = ok %v, %q; want the refusal naming the phrase", verb, ok, message)
				}
				if ok, message := do(t, svc, verb, first.ID, repointWeb); !ok {
					t.Fatalf("confirmed %s refused: %s", verb, message)
				}
			})
		}
	}
}

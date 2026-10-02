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
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func displayNameApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: appv1alpha1.AppSpec{
			DisplayName: "Original label",
			Image:       "nginx:1",
			Subdomain:   "stable-route",
			Host:        "custom.example.com",
			RestartedAt: "2026-07-13T00:00:00Z",
		},
		Status: appv1alpha1.AppStatus{URL: "https://stable-route.onbex.co"},
	}
}

func TestSetDisplayNameSetClearAndIdentityInvariant(t *testing.T) {
	svc, cl := newService(nil, displayNameApp("immutable-id"))

	got, err := svc.SetDisplayName(context.Background(), "immutable-id", "  Friendly API  ")
	if err != nil {
		t.Fatalf("SetDisplayName: %v", err)
	}
	if got.DisplayName != "Friendly API" {
		t.Fatalf("displayName = %q, want trimmed Friendly API", got.DisplayName)
	}
	a := getApp(t, cl, "immutable-id")
	if a.Name != "immutable-id" || a.Spec.Subdomain != "stable-route" || a.Spec.Host != "custom.example.com" {
		t.Fatalf("display-name change altered resource/routing identity: name=%q subdomain=%q host=%q", a.Name, a.Spec.Subdomain, a.Spec.Host)
	}
	if a.Spec.RestartedAt != "2026-07-13T00:00:00Z" || a.Status.URL != "https://stable-route.onbex.co" {
		t.Fatalf("display-name change triggered or rewrote runtime state: restartedAt=%q url=%q", a.Spec.RestartedAt, a.Status.URL)
	}

	got, err = svc.SetDisplayName(context.Background(), "immutable-id", "")
	if err != nil {
		t.Fatalf("clear SetDisplayName: %v", err)
	}
	if got.DisplayName != "" || getApp(t, cl, "immutable-id").Spec.DisplayName != "" {
		t.Fatalf("clear did not persist: view=%q spec=%q", got.DisplayName, getApp(t, cl, "immutable-id").Spec.DisplayName)
	}
}

// The rename has to reach the control-plane row, not just the CR: the
// workspace-wide event feed joins apps at dispatch time and has no CR to read,
// so a CR-only rename left every webhook and push notification reporting the
// creation-time name (w6/m101).
func TestSetDisplayNameManagedAppWritesRowThenCR(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, manage(displayNameApp("immutable-id"), "srv-1"))

	if _, err := svc.SetDisplayName(context.Background(), "immutable-id", "  Friendly API  "); err != nil {
		t.Fatalf("SetDisplayName: %v", err)
	}
	if len(rec.displayNameCalls) != 1 || rec.displayNameCalls[0].id != "srv-1" || rec.displayNameCalls[0].displayName != "Friendly API" {
		t.Fatalf("row write = %v, want [srv-1 \"Friendly API\"] (trimmed, same as the CR)", rec.displayNameCalls)
	}
	if got := getApp(t, cl, "immutable-id").Spec.DisplayName; got != "Friendly API" {
		t.Errorf("CR spec.displayName = %q, want the trimmed label", got)
	}

	// A whitespace-only value IS a clear: the trim happens before either write,
	// so the row never holds a value the feed's fallback would fail to catch.
	if _, err := svc.SetDisplayName(context.Background(), "immutable-id", "   "); err != nil {
		t.Fatalf("clear SetDisplayName: %v", err)
	}
	if len(rec.displayNameCalls) != 2 || rec.displayNameCalls[1].displayName != "" {
		t.Fatalf("clear row write = %v, want a trailing empty write", rec.displayNameCalls)
	}
	if got := getApp(t, cl, "immutable-id").Spec.DisplayName; got != "" {
		t.Errorf("CR spec.displayName = %q, want cleared", got)
	}
}

func TestSetDisplayNameUnmanagedAppSkipsStore(t *testing.T) {
	rec := &recordingStore{}
	app := displayNameApp("hand")
	app.Labels = map[string]string{core.LabelAppID: "srv-direct"} // an id alone is not store-managed
	svc, cl := newService(rec, app)

	if _, err := svc.SetDisplayName(context.Background(), "hand", "Hand rolled"); err != nil {
		t.Fatalf("SetDisplayName: %v", err)
	}
	if len(rec.displayNameCalls) != 0 {
		t.Errorf("unmanaged App wrote rows: %v", rec.displayNameCalls)
	}
	if got := getApp(t, cl, "hand").Spec.DisplayName; got != "Hand rolled" {
		t.Errorf("CR spec.displayName = %q, want the new label", got)
	}
}

func TestSetDisplayNameWithoutStorePatchesCR(t *testing.T) {
	svc, cl := newService(nil, displayNameApp("web"))

	if _, err := svc.SetDisplayName(context.Background(), "web", "Store-less"); err != nil {
		t.Fatalf("SetDisplayName: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.DisplayName; got != "Store-less" {
		t.Errorf("CR spec.displayName = %q, want the new label", got)
	}
}

// Half-applying the rename is what produced the wrong-serviceName bug, so a
// failed row write must surface rather than leave a CR the feed disagrees with.
func TestSetDisplayNameRowWriteFailureLeavesCRUntouched(t *testing.T) {
	rec := &recordingStore{err: errors.New("row write failed")}
	svc, cl := newService(rec, manage(displayNameApp("immutable-id"), "srv-1"))

	if _, err := svc.SetDisplayName(context.Background(), "immutable-id", "Friendly API"); err == nil {
		t.Fatal("SetDisplayName succeeded despite a failing row write")
	}
	if got := getApp(t, cl, "immutable-id").Spec.DisplayName; got != "Original label" {
		t.Errorf("CR spec.displayName = %q, want the untouched original", got)
	}
}

func TestSetDisplayNameRejectsNonMemberWithoutMutation(t *testing.T) {
	app := displayNameApp("other")
	app.Labels = map[string]string{core.LabelTenant: "tea-b", core.LabelServiceName: "other"}
	svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"}, app)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	// Round-7 F8: the by-name denial reports absence (name probes must not
	// distinguish a foreign App from a missing one).
	for _, value := range []string{"Forbidden rename", "\ninvalid rename"} {
		if _, err := svc.SetDisplayName(ctx, "other", value); !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("SetDisplayName by non-member = %v, want ErrNotFound", err)
		}
		if _, err := svc.ApplyServicePatch(ctx, "other", ServicePatch{DisplayName: &value}); !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("patch by non-member = %v, want ErrNotFound", err)
		}
	}
	if got := getApp(t, cl, "other").Spec.DisplayName; got != "Original label" {
		t.Fatalf("denied rename mutated displayName to %q", got)
	}
}

func TestRESTDisplayNamePatchRoundTripAndOmission(t *testing.T) {
	svc, cl := newService(nil, displayNameApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	patch := func(body string) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH %s => %d: %s", body, rec.Code, rec.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode PATCH response: %v", err)
		}
		return out
	}

	if out := patch(`{"displayName":"Customer API"}`); out["displayName"] != "Customer API" || out["name"] != "Customer API" || out["id"] != "web" {
		t.Fatalf("set response = %#v", out)
	}
	if out := patch(`{}`); out["displayName"] != "Customer API" {
		t.Fatalf("omitted displayName changed value: %#v", out)
	}
	if out := patch(`{"displayName":""}`); out["displayName"] != "" || out["name"] != "web" {
		t.Fatalf("clear response = %#v", out)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/web", nil))
	var out map[string]any
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out["displayName"] != "" {
		t.Fatalf("GET after clear => %d %s", rec.Code, rec.Body.String())
	}
	if a := getApp(t, cl, "web"); a.Name != "web" || a.Spec.Subdomain != "stable-route" {
		t.Fatalf("REST rename altered identity: %+v", a)
	}
}

func TestGraphQLDisplayNameRoundTrip(t *testing.T) {
	svc, _ := newService(nil, displayNameApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	run := func(query string) map[string]any {
		t.Helper()
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
		if len(res.Errors) > 0 {
			t.Fatalf("GraphQL %s: %v", query, res.Errors)
		}
		return res.Data.(map[string]any)
	}

	// w6/m115: GraphQL `name` is now the MUTABLE label (matching REST/MCP), and
	// the immutable name moved to `immutableName`. Before the fix, `name` here
	// read back "web" — the raw immutable name — so GraphQL disagreed with every
	// other read surface for a renamed service.
	set := run(`mutation { setDisplayName(id:"web", displayName:"Customer API") { id name immutableName displayName } }`)["setDisplayName"].(map[string]any)
	if set["id"] != "web" || set["name"] != "Customer API" || set["immutableName"] != "web" || set["displayName"] != "Customer API" {
		t.Fatalf("setDisplayName = %#v", set)
	}
	read := run(`{ server(id:"web") { id name immutableName displayName } }`)["server"].(map[string]any)
	if read["name"] != "Customer API" || read["immutableName"] != "web" || read["displayName"] != "Customer API" {
		t.Fatalf("server read = %#v", read)
	}
	clear := run(`mutation { setDisplayName(id:"web", displayName:"") { name displayName } }`)["setDisplayName"].(map[string]any)
	if clear["name"] != "web" || clear["displayName"] != "" {
		t.Fatalf("clear = %#v", clear)
	}
}

func TestMCPDisplayNameRoundTrip(t *testing.T) {
	svc, _ := newService(nil, displayNameApp("web"))
	ctx := context.Background()
	cs := displayNameMCPSession(t, svc)
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v isError=%v", name, err, res != nil && res.IsError)
		}
		b, _ := json.Marshal(res.StructuredContent)
		var out map[string]any
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		return out
	}

	set := call("update_service", map[string]any{"serviceId": "web", "displayName": "Customer API"})
	if set["name"] != "Customer API" || set["displayName"] != "Customer API" {
		t.Fatalf("update_service displayName = %#v", set)
	}
	read := call("get_service", map[string]any{"serviceId": "web"})
	if read["id"] != "web" || read["displayName"] != "Customer API" {
		t.Fatalf("get_service after rename = %#v", read)
	}
	// The empty string still CLEARS, which is the whole reason the folded
	// arguments are pointers: absent and empty must not mean the same thing.
	clear := call("update_service", map[string]any{"serviceId": "web", "displayName": ""})
	if clear["name"] != "web" || clear["displayName"] != "" {
		t.Fatalf("update_service displayName=\"\" = %#v", clear)
	}
	// An update_service call carrying no settable field is a read-only no-op,
	// not a clear-everything: the label survives.
	relabeled := call("update_service", map[string]any{"serviceId": "web", "displayName": "Customer API"})
	if relabeled["displayName"] != "Customer API" {
		t.Fatalf("re-set displayName = %#v", relabeled)
	}
	noop := call("update_service", map[string]any{"serviceId": "web"})
	if noop["id"] != "web" || noop["displayName"] != "Customer API" {
		t.Fatalf("no-op update_service = %#v", noop)
	}
}

// w8/m47: a rename to a name another service in the workspace was created as
// or is shown as got 200 — two services listed as "hello-go". The store
// refuses it; the verb answers exactly like create's duplicate (409 CONFLICT
// "name … is already in use", unprefixed) and the CR keeps its old name.
func TestSetDisplayNameTakenNameIsCreatesConflict(t *testing.T) {
	rec := &recordingStore{err: fmt.Errorf("service name %q: %w", "hello-go", store.ErrConflict)}
	svc, cl := newService(rec, manage(displayNameApp("immutable-id"), "srv-1"))

	_, err := svc.SetDisplayName(context.Background(), "immutable-id", "hello-go")
	var coded *core.CodedError
	if !errors.Is(err, core.ErrConflict) || !errors.As(err, &coded) || coded.Code != "CONFLICT" ||
		err.Error() != `name "hello-go" is already in use` {
		t.Fatalf("rename to a taken name = %v, want create's 409 CONFLICT", err)
	}
	if got := getApp(t, cl, "immutable-id").Spec.DisplayName; got != "Original label" {
		t.Errorf("CR spec.displayName = %q, want unchanged after the refusal", got)
	}
}

// w8/023: lifecycle writes on a service being deleted answer 404 like reads,
// before the row write or the CR patch.
func TestLifecycleWritesOnADeletingServiceAre404(t *testing.T) {
	rec := &recordingStore{}
	app := manage(displayNameApp("immutable-id"), "srv-1")
	now := metav1.Now()
	app.DeletionTimestamp = &now
	app.Finalizers = []string{"app.bex.co/finalizer"}
	svc, _ := newService(rec, app)

	for _, value := range []string{"renamed", "\ninvalid rename"} {
		if _, err := svc.SetDisplayName(context.Background(), "immutable-id", value); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("rename a deleting service = %v, want ErrNotFound", err)
		}
		if _, err := svc.ApplyServicePatch(context.Background(), "immutable-id", ServicePatch{DisplayName: &value}); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("patch a deleting service = %v, want ErrNotFound", err)
		}
	}
	if _, err := svc.Restart(context.Background(), "immutable-id"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("restart a deleting service = %v, want ErrNotFound", err)
	}
	if len(rec.displayNameCalls) != 0 {
		t.Errorf("row written for a deleting service: %v", rec.displayNameCalls)
	}
}

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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/graphql-go/graphql"
)

func TestNotificationsToSendRoundTripAndLegacyProjection(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	for _, tc := range []struct{ policy, notify string }{
		{"default", "default"}, {"failure", "notify"}, {"all", "notify"}, {"none", "ignore"},
	} {
		v, err := svc.SetNotificationsToSend(context.Background(), "web", tc.policy)
		if err != nil {
			t.Fatalf("SetNotificationsToSend(%q): %v", tc.policy, err)
		}
		if v.NotificationsToSend != tc.policy || v.NotifyOnFail != tc.notify {
			t.Errorf("view = (%q,%q), want (%q,%q)", v.NotificationsToSend, v.NotifyOnFail, tc.policy, tc.notify)
		}
		got := getApp(t, cl, "web").Spec
		if got.NotificationsToSend != tc.policy || got.NotifyOnFail != tc.notify {
			t.Errorf("spec = (%q,%q), want (%q,%q)", got.NotificationsToSend, got.NotifyOnFail, tc.policy, tc.notify)
		}
	}
	if _, err := svc.SetNotificationsToSend(context.Background(), "web", "sometimes"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("unknown policy error = %v", err)
	}
}

func TestNotificationOverrideRESTContract(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/notification-settings/overrides/services/web", strings.NewReader(`{"notificationsToSend":"all"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d: %s", rec.Code, rec.Body)
	}
	var got struct {
		NotificationsToSend string `json:"notificationsToSend"`
		Preview             string `json:"previewNotificationsEnabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.NotificationsToSend != "all" || got.Preview != "default" {
		t.Fatalf("response = %+v", got)
	}
	rec = httptest.NewRecorder()
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/notification-settings/overrides/services/web", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"notificationsToSend":"all"`) {
		t.Fatalf("GET = %d: %s", rec.Code, rec.Body)
	}
}

func TestNotificationsToSendGraphQL(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}), Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()})})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { setNotificationsToSend(id:"web", value:"failure") { notificationsToSend notifyOnFail } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("errors: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["setNotificationsToSend"].(map[string]any)
	if got["notificationsToSend"] != "failure" || got["notifyOnFail"] != "notify" {
		t.Fatalf("response = %+v", got)
	}
}

// w2/m100 t002: Render's workspace-wide override list. bex registered only the
// per-service pair and answered this with a bare `404 page not found`, while
// ADR018 claimed notifications parity on every surface and named every other
// divergence but not this one.
func TestNotificationOverridesListContract(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"), sampleApp("worker"), sampleApp("cron"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	patch := func(t *testing.T, service, policy string) {
		t.Helper()
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch,
			"/v1/notification-settings/overrides/services/"+service,
			strings.NewReader(`{"notificationsToSend":"`+policy+`"}`)))
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH %s = %d: %s", service, rec.Code, rec.Body)
		}
	}
	list := func(t *testing.T, query string) []notificationOverrideEntry {
		t.Helper()
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/notification-settings/overrides"+query, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %q = %d: %s", query, rec.Code, rec.Body)
		}
		var got []notificationOverrideEntry
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode %q: %v (%s)", query, err, rec.Body)
		}
		return got
	}
	byService := func(entries []notificationOverrideEntry) map[string]notificationOverrideItem {
		out := make(map[string]notificationOverrideItem, len(entries))
		for _, e := range entries {
			out[e.Override.ServiceID] = e.Override
		}
		return out
	}

	patch(t, "web", "failure")

	all := list(t, "")
	if len(all) != 3 {
		t.Fatalf("list returned %d entries, want all 3 services", len(all))
	}
	items := byService(all)
	// A patched service reports its policy; an untouched one is listed at
	// `default` rather than omitted, so an empty list means "no services", not
	// "no overrides" (the projection choice recorded in the m100 README).
	if got := items["web"]; got.NotificationsToSend != "failure" {
		t.Errorf("web = %+v, want notificationsToSend failure", got)
	}
	if got := items["worker"]; got.NotificationsToSend != "default" {
		t.Errorf("worker = %+v, want the default projection", got)
	}
	for id, item := range items {
		if item.Type != notificationOverrideTypeService {
			t.Errorf("%s type = %q, want %q", id, item.Type, notificationOverrideTypeService)
		}
		if item.PreviewNotificationsEnabled != "default" {
			t.Errorf("%s previewNotificationsEnabled = %q, want default", id, item.PreviewNotificationsEnabled)
		}
	}
	for _, e := range all {
		if e.Cursor == "" {
			t.Errorf("entry %s carries no cursor", e.Override.ServiceID)
		}
	}

	// Render's serviceId filter is an array parameter: it repeats.
	if got := list(t, "?serviceId=web"); len(got) != 1 || got[0].Override.ServiceID != "web" {
		t.Fatalf("?serviceId=web = %+v, want just web", got)
	}
	if got := list(t, "?serviceId=web&serviceId=cron"); len(got) != 2 {
		t.Fatalf("repeated serviceId = %d entries, want 2", len(got))
	}
	if got := list(t, "?serviceId=nope"); len(got) != 0 {
		t.Fatalf("unknown serviceId = %+v, want empty", got)
	}

	// Paging applies only when cursor or limit is asked for, matching the
	// sibling list routes' StablePage contract.
	first := list(t, "?limit=2")
	if len(first) != 2 {
		t.Fatalf("?limit=2 = %d entries, want 2", len(first))
	}
	next := list(t, "?limit=2&cursor="+first[len(first)-1].Cursor)
	if len(next) != 1 {
		t.Fatalf("second page = %d entries, want the remaining 1", len(next))
	}
	if next[0].Override.ServiceID == first[0].Override.ServiceID {
		t.Fatal("the second page repeated the first page's first entry")
	}
}

// The list must never become a cross-workspace read: it is scoped by the same
// s.List call every other collection uses, so a foreign ownerId resolves to
// nothing rather than to that workspace's services.
func TestNotificationOverridesListIsWorkspaceScoped(t *testing.T) {
	svc := &Service{Base: &core.Base{
		Client:    fakeClient(tenantApp("mine", "tea-a"), tenantApp("theirs", "tea-b")),
		Namespace: "default",
		Workspace: fakeWorkspace{"id-a": "tea-a"},
	}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	list := func(t *testing.T, query string) []notificationOverrideEntry {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, "/v1/notification-settings/overrides"+query, nil)
		r = r.WithContext(core.WithIdentity(r.Context(), core.Identity{Subject: "id-a"}))
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, r)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %q = %d: %s", query, rec.Code, rec.Body)
		}
		var got []notificationOverrideEntry
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode %q: %v (%s)", query, err, rec.Body)
		}
		return got
	}

	for _, tc := range []struct{ name, query string }{
		{"the caller's own workspace by default", ""},
		{"its own workspace named explicitly", "?ownerId=tea-a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := list(t, tc.query)
			if len(got) != 1 || got[0].Override.ServiceID != "mine" {
				t.Fatalf("= %+v, want only the caller's own service", got)
			}
		})
	}

	// The one that matters: naming someone else's workspace must not read it.
	t.Run("a foreign ownerId reads nothing", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/v1/notification-settings/overrides?ownerId=tea-b", nil)
		r = r.WithContext(core.WithIdentity(r.Context(), core.Identity{Subject: "id-a"}))
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, r)
		if strings.Contains(rec.Body.String(), "theirs") {
			t.Fatalf("a foreign workspace's service leaked: %d %s", rec.Code, rec.Body)
		}
	})
}

// Render requires serviceId on the single override object, as on the list's
// rows, and both single verbs answer it: the public service id, whatever
// selector the path used (w8/138).
func TestNotificationOverrideSingleObjectCarriesServiceID(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	view, err := svc.Get(context.Background(), "web")
	if err != nil || view.ID == "" {
		t.Fatalf("precondition: Get = %+v, %v", view, err)
	}
	want := view.ID
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/v1/notification-settings/overrides/services/web", nil),
		httptest.NewRequest(http.MethodPatch, "/v1/notification-settings/overrides/services/web", strings.NewReader(`{"notificationsToSend":"failure"}`)),
	} {
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, req)
		var got struct{ ServiceID, NotificationsToSend, PreviewNotificationsEnabled string }
		if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != http.StatusOK || err != nil ||
			got.ServiceID != want || got.NotificationsToSend == "" || got.PreviewNotificationsEnabled == "" {
			t.Fatalf("%s = %d %s, want serviceId %q with both settings", req.Method, rec.Code, rec.Body.String(), want)
		}
	}
}

// TestNotificationOverrideKeepsPreviewNotificationsDisabled pins w9/112:
// Render's Terraform provider sends every web service's notification override
// as {"notificationsToSend":"failure","previewNotificationsEnabled":"false"},
// Render answers 200 and reads "false" back, and the provider stores the
// answer. bex answered 400 "use default", so render_web_service could not be
// created. "false" is now kept and read back on every surface; "true" stays
// refused by name, because preview notifications belong to PR previews.
func TestNotificationOverrideKeepsPreviewNotificationsDisabled(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"), sampleApp("worker"))
	view, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	id := view.ID
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req = req.WithContext(core.WithStrictJSONDecoding(req.Context()))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, req)
		return rec
	}
	override := func(rec *httptest.ResponseRecorder) notificationOverrideResponse {
		t.Helper()
		var got notificationOverrideResponse
		if rec.Code != http.StatusOK {
			t.Fatalf("override = %d: %s", rec.Code, rec.Body)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	serviceField := func(raw []byte) any {
		t.Helper()
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("decode service %s: %v", raw, err)
		}
		return got["previewNotificationsEnabled"]
	}
	path := "/v1/notification-settings/overrides/services/" + id

	// The provider's recorded PATCH, answered as Render answers it.
	want := notificationOverrideResponse{ServiceID: id, NotificationsToSend: "failure", PreviewNotificationsEnabled: "false"}
	if got := override(do(http.MethodPatch, path, `{"notificationsToSend":"failure","previewNotificationsEnabled":"false"}`)); got != want {
		t.Fatalf("PATCH = %+v, want %+v", got, want)
	}
	if spec := getApp(t, cl, "web").Spec; spec.PreviewNotificationsEnabled != "false" || spec.NotificationsToSend != "failure" {
		t.Fatalf("spec = (%q, %q), want (failure, false)", spec.NotificationsToSend, spec.PreviewNotificationsEnabled)
	}
	if got := override(do(http.MethodGet, path, "")); got != want {
		t.Errorf("GET = %+v, want %+v", got, want)
	}
	var list []notificationOverrideEntry
	if err := json.Unmarshal(do(http.MethodGet, "/v1/notification-settings/overrides", "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	for _, e := range list {
		wantPreview := "default"
		if e.Override.ServiceID == id {
			wantPreview = "false"
		}
		if e.Override.PreviewNotificationsEnabled != wantPreview {
			t.Errorf("list %s previewNotificationsEnabled = %q, want %q", e.Override.ServiceID, e.Override.PreviewNotificationsEnabled, wantPreview)
		}
	}
	if got := serviceField(do(http.MethodGet, "/v1/services/"+id, "").Body.Bytes()); got != "false" {
		t.Errorf("REST service previewNotificationsEnabled = %v, want false", got)
	}
	res := callMCPTool(t, svc, "get_service", map[string]any{"serviceId": id})
	if res.IsError {
		t.Fatalf("MCP get_service: %s", mcpResultText(res))
	}
	if got := serviceField([]byte(mcpResultText(res))); got != "false" {
		t.Errorf("MCP get_service previewNotificationsEnabled = %v, want false", got)
	}
	schema := gqlSchema(t, svc)
	readGQL := func() any {
		t.Helper()
		return gqlDo(t, schema, `{ service(id: "`+id+`") { previewNotificationsEnabled } }`)["service"].(map[string]any)["previewNotificationsEnabled"]
	}
	if got := readGQL(); got != "false" {
		t.Errorf("GraphQL previewNotificationsEnabled = %v, want false", got)
	}

	// "true" is refused by name on every surface, and a refused PATCH writes
	// neither field: both are checked before either is written.
	rec := do(http.MethodPatch, path, `{"notificationsToSend":"all","previewNotificationsEnabled":"true"}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "NOTIFICATION_PREVIEWS_UNSUPPORTED") ||
		!strings.Contains(rec.Body.String(), "previewNotificationsEnabled cannot be true") {
		t.Errorf("PATCH true = %d %s, want 400 NOTIFICATION_PREVIEWS_UNSUPPORTED naming the field", rec.Code, rec.Body)
	}
	if spec := getApp(t, cl, "web").Spec; spec.PreviewNotificationsEnabled != "false" || spec.NotificationsToSend != "failure" {
		t.Errorf("after a refused PATCH spec = (%q, %q), want it unchanged (failure, false)", spec.NotificationsToSend, spec.PreviewNotificationsEnabled)
	}
	gqlRes := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setPreviewNotificationsEnabled(id: "` + id + `", value: "true") { id } }`})
	if len(gqlRes.Errors) == 0 || !strings.Contains(gqlRes.Errors[0].Message, "previewNotificationsEnabled cannot be true") {
		t.Errorf("GraphQL true = %v, want the refusal", gqlRes.Errors)
	}
	if res := callMCPTool(t, svc, "update_service", map[string]any{"serviceId": id, "previewNotificationsEnabled": "true"}); !res.IsError ||
		!strings.Contains(mcpResultText(res), "previewNotificationsEnabled cannot be true") {
		t.Errorf("MCP true = %s, want the refusal", mcpResultText(res))
	}
	if _, err := svc.SetPreviewNotificationsEnabled(context.Background(), "web", "maybe"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("an unknown value = %v, want a 400", err)
	}

	// Back to default: the override reads "default" and the service object
	// drops its extension again.
	gqlDo(t, schema, `mutation { setPreviewNotificationsEnabled(id: "`+id+`", value: "default") { id } }`)
	if got := readGQL(); got != "default" {
		t.Errorf("GraphQL after default = %v, want default", got)
	}
	if got := serviceField(do(http.MethodGet, "/v1/services/"+id, "").Body.Bytes()); got != nil {
		t.Errorf("REST service at default carries previewNotificationsEnabled %v, want it omitted", got)
	}
	if res := callMCPTool(t, svc, "update_service", map[string]any{"serviceId": id, "previewNotificationsEnabled": "false"}); res.IsError ||
		serviceField([]byte(mcpResultText(res))) != "false" {
		t.Errorf("MCP update_service false = %s, want it read back", mcpResultText(res))
	}
	if got := override(do(http.MethodGet, path, "")); got != want {
		t.Errorf("GET after MCP = %+v, want %+v", got, want)
	}
}

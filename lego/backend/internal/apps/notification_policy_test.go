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
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/notification-settings/overrides/services/web", strings.NewReader(`{"notificationsToSend":"all"}`)))
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
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/notification-settings/overrides/services/web", nil))
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

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
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/591: GraphQL names a cancel reason by its template, so the dashboard can
// word it in its own language; a user cancel carries none. REST keeps the
// English sentence, as Render's API does.
func TestCancelReasonTemplateOnGraphQL(t *testing.T) {
	at := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	ds := newFakeStore()
	ds.byApp["srv-1"] = []store.Deploy{
		{ID: "dep-old", AppID: "srv-1", Trigger: store.TriggerAPI, Status: store.DeployCanceled,
			CancelReason: "Superseded by dep-new", CreatedAt: at.Add(-time.Hour), UpdatedAt: at},
		{ID: "dep-user", AppID: "srv-1", Trigger: store.TriggerAPI, Status: store.DeployCanceled, CreatedAt: at, UpdatedAt: at},
	}
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	res := graphql.Do(graphql.Params{Schema: testSchema(t, svc), Context: context.Background(), RequestString: `{
		old: deploy(serviceId: "web", deployId: "dep-old") { cancelReasonTemplate { code params { name value } } }
		user: deploy(serviceId: "web", deployId: "dep-user") { cancelReasonTemplate { code } }
	}`})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL: %v", res.Errors)
	}
	data := res.Data.(map[string]any)
	want := map[string]any{"code": "superseded_by", "params": []any{map[string]any{"name": "deploy", "value": "dep-new"}}}
	if got := data["old"].(map[string]any)["cancelReasonTemplate"]; !reflect.DeepEqual(got, want) {
		t.Errorf("superseded cancelReasonTemplate = %v, want %v", got, want)
	}
	if got := data["user"].(map[string]any)["cancelReasonTemplate"]; got != nil {
		t.Errorf("user cancel cancelReasonTemplate = %v, want null", got)
	}

	var rest map[string]any
	if err := json.Unmarshal(newRESTHarness(t, svc)("GET", "/v1/services/web/deploys/dep-old").Body.Bytes(), &rest); err != nil {
		t.Fatal(err)
	}
	if rest["cancelReason"] != "Superseded by dep-new" {
		t.Errorf("REST cancelReason = %v, want the English sentence", rest["cancelReason"])
	}
}

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

package webhooks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// deliveryPayloadCases are the stored request bodies a delivery read has to
// survive: the current shape, the pre-serviceName shape, and every way a
// persisted blob can be unusable. The expectation is always a string — the
// read must degrade to "", never to an error, because the history of a
// webhook endpoint is evidence about past sends and may legitimately contain
// bodies this build never wrote.
var deliveryPayloadCases = []struct {
	name    string
	payload string
	want    string
}{
	{"current shape", `{"type":"deploy_ended","timestamp":"2026-08-16T12:00:00Z","data":{"id":"evt-1","serviceId":"srv-api","serviceName":"api"}}`, "api"},
	{"older event shape without the field", `{"type":"deploy_ended","data":{"id":"evt-1","serviceId":"srv-api"}}`, ""},
	{"no data object at all", `{"type":"deploy_ended"}`, ""},
	{"empty body", ``, ""},
	{"truncated JSON", `{"type":"deploy_ended","data":{"serviceName":"ap`, ""},
	{"not JSON", `<html>gateway timeout</html>`, ""},
	{"JSON but not an object", `["deploy_ended"]`, ""},
	{"data is the wrong JSON type", `{"data":"srv-api"}`, ""},
	{"serviceName is the wrong JSON type", `{"data":{"serviceName":42}}`, ""},
	{"blank recorded name", `{"data":{"serviceName":"   "}}`, ""},
}

// TestDeliveryReadsAgreeOnRecordedServiceName is w2/m96/t004's contract: the
// recorded subject name is parsed once on the server, so REST's delivery
// history and the GraphQL WebhookDelivery type report the SAME value for the
// same stored attempt. Parsing in each client instead is exactly how the two
// surfaces would drift.
func TestDeliveryReadsAgreeOnRecordedServiceName(t *testing.T) {
	for _, tc := range deliveryPayloadCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, st := newTestService()
			created, err := svc.Create(t.Context(), CreateRequest{
				Name: "names", URL: "https://hooks.example.com", EventTypes: []string{}, Enabled: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			sent := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
			st.deliveries[created.ID] = []store.WebhookAttempt{{
				ID: "whd-name", NotificationID: "whd-name-parent", EndpointID: created.ID,
				EventID: "evt-1", EventType: TypeDeployEnded, ServiceID: "srv-api",
				Status: store.WebhookAttemptDelivered, AttemptNumber: 1, StatusCode: 204,
				Payload: tc.payload, SentAt: &sent,
				ParentStatus: store.WebhookAttemptDelivered, CreatedAt: sent,
			}}

			rr := webhookREST(t, svc, http.MethodGet, "/v1/webhooks/"+created.ID+"/events", "")
			if rr.Code != http.StatusOK {
				t.Fatalf("REST history = %d %s", rr.Code, rr.Body.String())
			}
			var page []struct {
				WebhookEvent map[string]any `json:"webhookEvent"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &page); err != nil || len(page) != 1 {
				t.Fatalf("REST history page = %s, err %v", rr.Body.String(), err)
			}
			restName, present := page[0].WebhookEvent["serviceName"]
			if !present {
				t.Fatalf("REST history omitted serviceName entirely: %v", page[0].WebhookEvent)
			}
			if restName != any(tc.want) {
				t.Errorf("REST serviceName = %#v, want %q", restName, tc.want)
			}

			result := graphql.Do(graphql.Params{
				Schema: webhookGraphQLSchema(t, svc), Context: t.Context(),
				RequestString: fmt.Sprintf(`{ webhookDeliveries(endpointId:%q) { id serviceId serviceName } }`, created.ID),
			})
			if len(result.Errors) != 0 {
				t.Fatalf("GraphQL history errors: %v", result.Errors)
			}
			raw, _ := json.Marshal(result.Data)
			var got struct {
				Deliveries []struct {
					ID          string `json:"id"`
					ServiceID   string `json:"serviceId"`
					ServiceName string `json:"serviceName"`
				} `json:"webhookDeliveries"`
			}
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Deliveries) != 1 {
				t.Fatalf("GraphQL history = %+v, want one delivery", got.Deliveries)
			}
			if got.Deliveries[0].ServiceName != tc.want {
				t.Errorf("GraphQL serviceName = %q, want %q", got.Deliveries[0].ServiceName, tc.want)
			}
			if got.Deliveries[0].ServiceName != restName {
				t.Errorf("surfaces disagree: GraphQL %q vs REST %#v", got.Deliveries[0].ServiceName, restName)
			}
			// The id stays addressable alongside the recorded name — the
			// dashboard renders the name as the link text and the id beneath it.
			if got.Deliveries[0].ServiceID != "srv-api" {
				t.Errorf("serviceId = %q, want srv-api", got.Deliveries[0].ServiceID)
			}
		})
	}
}

// TestRecordedServiceNameSurvivesARename pins WHY the name is read back out of
// the delivered payload rather than resolved live: the attempt must keep
// reporting what bex actually told the receiver, even once the service is
// renamed or deleted and no live lookup could answer at all.
func TestRecordedServiceNameSurvivesARename(t *testing.T) {
	const beforeRename = `{"type":"deploy_ended","data":{"id":"evt-1","serviceId":"srv-api","serviceName":"api-old"}}`
	const afterRename = `{"type":"deploy_ended","data":{"id":"evt-2","serviceId":"srv-api","serviceName":"api-new"}}`

	svc, st := newTestService()
	created, err := svc.Create(t.Context(), CreateRequest{
		Name: "renames", URL: "https://hooks.example.com", EventTypes: []string{}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	for i, payload := range []string{beforeRename, afterRename} {
		sent := base.Add(time.Duration(i) * time.Hour)
		st.deliveries[created.ID] = append(st.deliveries[created.ID], store.WebhookAttempt{
			ID: fmt.Sprintf("whd-%d", i), NotificationID: fmt.Sprintf("whd-parent-%d", i),
			EndpointID: created.ID, EventID: fmt.Sprintf("evt-%d", i+1), EventType: TypeDeployEnded,
			ServiceID: "srv-api", Status: store.WebhookAttemptDelivered, AttemptNumber: 1,
			StatusCode: 204, Payload: payload, SentAt: &sent,
			ParentStatus: store.WebhookAttemptDelivered, CreatedAt: sent,
		})
	}

	views, err := svc.ListDeliveriesFiltered(t.Context(), "", created.ID, DeliveryFilter{})
	if err != nil {
		t.Fatalf("ListDeliveriesFiltered: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("history = %d rows, want 2", len(views))
	}
	byID := map[string]string{}
	for _, v := range views {
		byID[v.ID] = v.ServiceName
	}
	if byID["whd-0"] != "api-old" || byID["whd-1"] != "api-new" {
		t.Errorf("recorded names = %v, want whd-0 api-old and whd-1 api-new", byID)
	}
}

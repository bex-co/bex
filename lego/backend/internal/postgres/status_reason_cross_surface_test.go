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

package postgres

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/graphql-go/graphql"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestStatusReasonCodeParityAcrossSurfaces pins w5/079: an unavailable
// database's reason code and sentence read the same on REST, GraphQL and MCP,
// so every client can translate the same code; any other database carries
// neither (GraphQL null, REST and MCP omitted).
func TestStatusReasonCodeParityAcrossSurfaces(t *testing.T) {
	svc, cl := newService()
	created := serveREST(svc, http.MethodPost, "/v1/postgres", `{"name":"reason-pg","plan":"free"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", created.Code, created.Body.String())
	}
	id, _ := decodeMap(t, created.Body.Bytes())["id"].(string)
	schema, err := pgGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	call, cleanup := pgMCPClient(t, svc)
	defer cleanup()
	surfaces := func() map[string]map[string]any {
		got := serveREST(svc, http.MethodGet, "/v1/postgres/"+id, "")
		if got.Code != http.StatusOK {
			t.Fatalf("GET => %d: %s", got.Code, got.Body.String())
		}
		res := graphql.Do(graphql.Params{
			Schema:        schema,
			Context:       context.Background(),
			RequestString: fmt.Sprintf(`{ database(id: %q) { status statusReason statusReasonCode } }`, id),
		})
		if len(res.Errors) > 0 {
			t.Fatalf("GraphQL errors: %v", res.Errors)
		}
		gql, _ := res.Data.(map[string]any)["database"].(map[string]any)
		return map[string]map[string]any{
			"REST":    decodeMap(t, got.Body.Bytes()),
			"GraphQL": gql,
			"MCP":     call("get_postgres", map[string]any{"postgresId": id}),
		}
	}

	for surface, got := range surfaces() {
		reason, hasReason := got["statusReason"]
		code, hasCode := got["statusReasonCode"]
		if surface == "GraphQL" && (reason != nil || code != nil) ||
			surface != "GraphQL" && (hasReason || hasCode) {
			t.Errorf("%s carries a reason for a %v database: %v / %v", surface, got["status"], reason, code)
		}
	}

	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: id}, &db); err != nil {
		t.Fatalf("get %s: %v", id, err)
	}
	db.Status.Phase = appv1alpha1.DBPhaseFailed
	meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: appv1alpha1.ReasonPoolerFailed, Message: "raw pooler error",
	})
	if err := cl.Update(context.Background(), &db); err != nil {
		t.Fatalf("fail %s: %v", id, err)
	}
	const sentence = "The connection pooler could not be provisioned."
	for surface, got := range surfaces() {
		if got["status"] != "unavailable" || got["statusReason"] != sentence || got["statusReasonCode"] != appv1alpha1.ReasonPoolerFailed {
			t.Errorf("%s = status %v, reason %v, code %v; want unavailable, %q, %s",
				surface, got["status"], got["statusReason"], got["statusReasonCode"], sentence, appv1alpha1.ReasonPoolerFailed)
		}
	}
}

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

package keyvalue

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

// TestStatusReasonCodeParityAcrossSurfaces pins w5/m129: an unavailable Key
// Value's reason code and sentence read the same on REST, GraphQL and MCP, so
// every client can translate the same code, and the condition's raw message is
// never published. Any other Key Value carries neither (GraphQL null, REST and
// MCP omitted).
func TestStatusReasonCodeParityAcrossSurfaces(t *testing.T) {
	svc, cl := newService()
	created := serveREST(svc, http.MethodPost, "/v1/key-value", `{"name":"reason-kv","plan":"free"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", created.Code, created.Body.String())
	}
	id, _ := decodeMap(t, created.Body.Bytes())["id"].(string)
	schema, err := kvGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	call, cleanup := kvMCPClient(t, svc)
	defer cleanup()
	surfaces := func() map[string]map[string]any {
		got := serveREST(svc, http.MethodGet, "/v1/key-value/"+id, "")
		if got.Code != http.StatusOK {
			t.Fatalf("GET => %d: %s", got.Code, got.Body.String())
		}
		res := graphql.Do(graphql.Params{
			Schema:        schema,
			Context:       context.Background(),
			RequestString: fmt.Sprintf(`{ keyValue(id: %q) { status statusReason statusReasonCode } }`, id),
		})
		if len(res.Errors) > 0 {
			t.Fatalf("GraphQL errors: %v", res.Errors)
		}
		gql, _ := res.Data.(map[string]any)["keyValue"].(map[string]any)
		return map[string]map[string]any{
			"REST":    decodeMap(t, got.Body.Bytes()),
			"GraphQL": gql,
			"MCP":     call("get_key_value", map[string]any{"keyValueId": id}),
		}
	}

	for surface, got := range surfaces() {
		reason, hasReason := got["statusReason"]
		code, hasCode := got["statusReasonCode"]
		if surface == "GraphQL" && (reason != nil || code != nil) ||
			surface != "GraphQL" && (hasReason || hasCode) {
			t.Errorf("%s carries a reason for a %v Key Value: %v / %v", surface, got["status"], reason, code)
		}
	}

	var kv appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: id}, &kv); err != nil {
		t.Fatalf("get %s: %v", id, err)
	}
	kv.Status.Phase = appv1alpha1.KVPhaseFailed
	meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: appv1alpha1.ReasonBackupCronJobFailed, Message: "raw cronjob error",
	})
	if err := cl.Update(context.Background(), &kv); err != nil {
		t.Fatalf("fail %s: %v", id, err)
	}
	const sentence = "The daily backup schedule could not be updated."
	for surface, got := range surfaces() {
		if got["status"] != "unavailable" || got["statusReason"] != sentence || got["statusReasonCode"] != appv1alpha1.ReasonBackupCronJobFailed {
			t.Errorf("%s = status %v, reason %v, code %v; want unavailable, %q, %s",
				surface, got["status"], got["statusReason"], got["statusReasonCode"], sentence, appv1alpha1.ReasonBackupCronJobFailed)
		}
	}
}

// TestEveryFailedReasonHasASentence (w5/m129): every reason the operator fails
// a Key Value with (appv1alpha1.KeyValueFailedReasons, which the operator's
// TestKeyValueReasonsAreClassified keeps complete) has a fixed sentence, except
// the storage shrink refusal, whose operator-authored message is published.
func TestEveryFailedReasonHasASentence(t *testing.T) {
	for _, reason := range appv1alpha1.KeyValueFailedReasons {
		if reason == appv1alpha1.ReasonStorageShrinkRejected {
			continue
		}
		if unavailableReasons[reason] == "" {
			t.Errorf("%s fails a Key Value but has no sentence", reason)
		}
	}
	if len(unavailableReasons) != len(appv1alpha1.KeyValueFailedReasons)-1 {
		t.Errorf("%d sentences for %d failed reasons: a sentence names a reason the operator no longer fails with",
			len(unavailableReasons), len(appv1alpha1.KeyValueFailedReasons)-1)
	}
}

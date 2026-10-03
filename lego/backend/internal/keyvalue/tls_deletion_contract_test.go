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
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestKeyValueDeleteAcknowledgesPendingTLSCleanup(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL"} {
		t.Run(surface, func(t *testing.T) {
			kv := keyValueForProtection("red-tls-delete", "tls-cache", false)
			kv.Spec.Public = true
			kv.Finalizers = []string{"app.bex.co/kv-tls-cleanup"}
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name: kv.Name + "-kv-tls", Namespace: kv.Namespace, UID: "issued-tls-uid",
			}, Type: corev1.SecretTypeTLS}
			svc, cl := newService(kv, secret)
			ctx := context.Background()
			schema, err := kvGQLSchema(svc)
			if err != nil {
				t.Fatal(err)
			}
			gqlutil.NilOnError(&schema)
			if surface == "REST" {
				got := serveREST(svc, http.MethodDelete, "/v1/key-value/"+kv.Name, "")
				if got.Code != http.StatusNoContent || got.Body.Len() != 0 {
					t.Fatalf("delete acknowledgment = %d %s", got.Code, got.Body.String())
				}
			} else {
				got := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
					RequestString: `mutation { deleteKeyValue(id: "red-tls-delete") }`})
				if len(got.Errors) != 0 || got.Data.(map[string]any)["deleteKeyValue"] != true {
					t.Fatalf("delete acknowledgment = %#v", got)
				}
			}

			var pending appv1alpha1.KeyValue
			if err := cl.Get(ctx, client.ObjectKeyFromObject(kv), &pending); err != nil || pending.DeletionTimestamp.IsZero() {
				t.Fatalf("TLS-finalized CR must remain physically pending: %v", err)
			}
			var issued corev1.Secret
			if err := cl.Get(ctx, client.ObjectKeyFromObject(secret), &issued); err != nil || issued.UID != secret.UID {
				t.Fatalf("API must leave physical TLS cleanup to the operator: %v", err)
			}
			if got := serveREST(svc, http.MethodGet, "/v1/key-value/"+kv.Name, ""); got.Code != http.StatusNotFound {
				t.Fatalf("pending REST read = %d", got.Code)
			}
			rows, err := svc.ListKeyValues(ctx, "")
			if err != nil || len(rows) != 0 {
				t.Fatalf("pending list = %v, %v", rows, err)
			}
			got := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ keyValue(id: "red-tls-delete") { id } }`})
			if len(got.Errors) == 0 || !strings.Contains(got.Errors[0].Message, "not found") || got.Data.(map[string]any)["keyValue"] != nil {
				t.Fatalf("pending GraphQL read = %#v", got)
			}
			server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
			svc.RegisterMCP(server)
			serverTransport, clientTransport := mcp.NewInMemoryTransports()
			serverSession, err := server.Connect(ctx, serverTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = serverSession.Close() })
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cs.Close() })
			result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_key_value", Arguments: map[string]any{"keyValueId": kv.Name}})
			if err != nil || !result.IsError {
				t.Fatalf("pending MCP read = %#v, %v", result, err)
			}
		})
	}
}

func TestRefusedKeyValueDeletePreservesTLSArtifacts(t *testing.T) {
	for _, refusal := range []string{"protection", "revocation"} {
		t.Run(refusal, func(t *testing.T) {
			kv := keyValueForProtection("red-tls-denied", "protected-cache", refusal == "protection")
			kv.Spec.Public = true
			kv.Finalizers = []string{"app.bex.co/kv-tls-cleanup"}
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name: kv.Name + "-kv-tls", Namespace: kv.Namespace, UID: "issued-tls-uid",
			}, Type: corev1.SecretTypeTLS}
			svc, cl, _ := protectedKeyValueService(kv, secret)
			want := core.ErrBadRequest
			if refusal == "revocation" {
				svc.Authz = staleAllowChecker{}
				want = core.ErrForbidden
			}
			ctx := ctxAs("user-a")
			if err := svc.DeleteKeyValue(ctx, kv.Name); !errors.Is(err, want) {
				t.Fatalf("refusal = %v, want %v", err, want)
			}
			var live appv1alpha1.KeyValue
			if err := cl.Get(ctx, client.ObjectKeyFromObject(kv), &live); err != nil || !live.DeletionTimestamp.IsZero() {
				t.Fatalf("refusal changed CR lifetime: %v", err)
			}
			var issued corev1.Secret
			if err := cl.Get(ctx, client.ObjectKeyFromObject(secret), &issued); err != nil || issued.UID != secret.UID {
				t.Fatalf("refusal changed TLS artifact: %v", err)
			}
		})
	}
}

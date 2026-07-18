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
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type fakeProtectionStore struct {
	statuses map[string]string
	calls    int
}

func (s *fakeProtectionStore) GetEnvironmentProtectedStatus(_ context.Context, environmentID string) (string, error) {
	s.calls++
	if status := s.statuses[environmentID]; status != "" {
		return status, nil
	}
	return core.ProtectedStatusUnprotected, nil
}

func keyValueForProtection(id, name string, protected bool) *appv1alpha1.KeyValue {
	labels := map[string]string{}
	if protected {
		labels[core.LabelEnvironment] = "env-production"
	}
	return &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: id, Namespace: "default", Labels: labels},
		Spec:       appv1alpha1.KeyValueSpec{Name: name, Plan: "free"},
	}
}

func protectedKeyValueService(objects ...client.Object) (*Service, client.Client, *fakeProtectionStore) {
	svc, cl := newService(objects...)
	store := &fakeProtectionStore{statuses: map[string]string{"env-production": core.ProtectedStatusProtected}}
	svc.Protection = store
	return svc, cl, store
}

func TestProtectedKeyValueDeleteAndSuspend(t *testing.T) {
	t.Run("delete blocks then accepts exact phrase", func(t *testing.T) {
		kv := keyValueForProtection("red-sessions", "sessions", true)
		svc, cl, _ := protectedKeyValueService(kv)
		if err := svc.DeleteKeyValue(context.Background(), kv.Name); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), `confirm="sudo delete key value sessions"`) {
			t.Fatalf("blocked DeleteKeyValue = %v", err)
		}
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(kv), &appv1alpha1.KeyValue{}); err != nil {
			t.Fatalf("blocked delete removed KeyValue: %v", err)
		}
		ctx := core.WithConfirm(context.Background(), ProtectedConfirmation("delete", "sessions"))
		if err := svc.DeleteKeyValue(ctx, kv.Name); err != nil {
			t.Fatalf("confirmed DeleteKeyValue: %v", err)
		}
	})

	t.Run("suspend blocks, confirms, and resume is exempt", func(t *testing.T) {
		kv := keyValueForProtection("red-sessions", "sessions", true)
		svc, cl, _ := protectedKeyValueService(kv)
		if _, err := svc.Suspend(context.Background(), kv.Name); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("blocked Suspend = %v", err)
		}
		ctx := core.WithConfirm(context.Background(), ProtectedConfirmation("suspend", "sessions"))
		if _, err := svc.Suspend(ctx, kv.Name); err != nil {
			t.Fatalf("confirmed Suspend: %v", err)
		}
		var got appv1alpha1.KeyValue
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(kv), &got); err != nil || !got.Spec.Suspended {
			t.Fatalf("KeyValue not suspended: suspended=%v err=%v", got.Spec.Suspended, err)
		}
		if _, err := svc.Resume(context.Background(), kv.Name); err != nil {
			t.Fatalf("Resume must not be protected: %v", err)
		}
	})

	t.Run("environment-less resource skips store", func(t *testing.T) {
		kv := keyValueForProtection("red-dev", "dev", false)
		svc, _, store := protectedKeyValueService(kv)
		if _, err := svc.Suspend(context.Background(), kv.Name); err != nil {
			t.Fatalf("environment-less Suspend: %v", err)
		}
		if store.calls != 0 {
			t.Fatalf("protection store calls = %d, want 0", store.calls)
		}
	})

	t.Run("unwired protection store is a no-op", func(t *testing.T) {
		kv := keyValueForProtection("red-hand-applied", "hand-applied", true)
		svc, _ := newService(kv)
		if _, err := svc.Suspend(context.Background(), kv.Name); err != nil {
			t.Fatalf("DB-less Suspend: %v", err)
		}
	})

	t.Run("unprotected environment needs no confirmation", func(t *testing.T) {
		kv := keyValueForProtection("red-staging", "staging", true)
		svc, _, store := protectedKeyValueService(kv)
		store.statuses["env-production"] = core.ProtectedStatusUnprotected
		if _, err := svc.Suspend(context.Background(), kv.Name); err != nil {
			t.Fatalf("unprotected Suspend: %v", err)
		}
	})
}

func TestProtectedKeyValueConfirmationAcrossAdapters(t *testing.T) {
	t.Run("REST delete and suspend", func(t *testing.T) {
		kv := keyValueForProtection("red-rest", "rest-kv", true)
		toDelete := keyValueForProtection("red-rest-delete", "rest-delete", true)
		svc, _, _ := protectedKeyValueService(kv, toDelete)
		if got := serveREST(svc, http.MethodPost, "/v1/key-value/red-rest/suspend", ""); got.Code != http.StatusBadRequest {
			t.Fatalf("unconfirmed suspend status = %d: %s", got.Code, got.Body.String())
		}
		confirm := url.QueryEscape(ProtectedConfirmation("suspend", "rest-kv"))
		if got := serveREST(svc, http.MethodPost, "/v1/key-value/red-rest/suspend?confirm="+confirm, ""); got.Code != http.StatusAccepted {
			t.Fatalf("confirmed suspend status = %d: %s", got.Code, got.Body.String())
		}
		if got := serveREST(svc, http.MethodDelete, "/v1/key-value/red-rest-delete", ""); got.Code != http.StatusBadRequest {
			t.Fatalf("unconfirmed delete status = %d: %s", got.Code, got.Body.String())
		}
		deleteConfirm := url.QueryEscape(ProtectedConfirmation("delete", "rest-delete"))
		if got := serveREST(svc, http.MethodDelete, "/v1/key-value/red-rest-delete?confirm="+deleteConfirm, ""); got.Code != http.StatusNoContent {
			t.Fatalf("confirmed delete status = %d: %s", got.Code, got.Body.String())
		}
	})

	t.Run("GraphQL delete", func(t *testing.T) {
		kv := keyValueForProtection("red-gql", "gql-kv", true)
		toSuspend := keyValueForProtection("red-gql-suspend", "gql-suspend", true)
		svc, _, _ := protectedKeyValueService(kv, toSuspend)
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		blocked := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { deleteKeyValue(id:"red-gql") }`})
		if len(blocked.Errors) == 0 || !strings.Contains(blocked.Errors[0].Message, "sudo delete key value gql-kv") {
			t.Fatalf("unconfirmed GraphQL errors = %v", blocked.Errors)
		}
		confirmed := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { deleteKeyValue(id:"red-gql", confirm:"sudo delete key value gql-kv") }`})
		if len(confirmed.Errors) != 0 {
			t.Fatalf("confirmed GraphQL errors = %v", confirmed.Errors)
		}
		blockedSuspend := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { suspendKeyValue(id:"red-gql-suspend") { id } }`})
		if len(blockedSuspend.Errors) == 0 || !strings.Contains(blockedSuspend.Errors[0].Message, "sudo suspend key value gql-suspend") {
			t.Fatalf("unconfirmed GraphQL suspend errors = %v", blockedSuspend.Errors)
		}
		confirmedSuspend := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { suspendKeyValue(id:"red-gql-suspend", confirm:"sudo suspend key value gql-suspend") { id } }`})
		if len(confirmedSuspend.Errors) != 0 {
			t.Fatalf("confirmed GraphQL suspend errors = %v", confirmedSuspend.Errors)
		}
	})

	t.Run("MCP suspend", func(t *testing.T) {
		kv := keyValueForProtection("red-mcp", "mcp-kv", true)
		svc, _, _ := protectedKeyValueService(kv)
		srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
		svc.RegisterMCP(srv)
		ctx := context.Background()
		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
			t.Fatal(err)
		}
		clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer clientSession.Close()
		tools, err := clientSession.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		var suspendSchema, getSchema string
		for _, tool := range tools.Tools {
			encoded, _ := json.Marshal(tool.InputSchema)
			switch tool.Name {
			case "suspend_keyvalue":
				suspendSchema = string(encoded)
			case "get_key_value":
				getSchema = string(encoded)
			}
		}
		if !strings.Contains(suspendSchema, `"confirm"`) || strings.Contains(getSchema, `"confirm"`) {
			t.Fatalf("confirm schema scope: suspend=%s get=%s", suspendSchema, getSchema)
		}
		blocked, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "suspend_keyvalue", Arguments: map[string]any{"keyValueId": kv.Name}})
		if err != nil || !blocked.IsError {
			t.Fatalf("unconfirmed MCP = %#v, %v", blocked, err)
		}
		confirmed, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "suspend_keyvalue", Arguments: map[string]any{"keyValueId": kv.Name, "confirm": ProtectedConfirmation("suspend", "mcp-kv")}})
		if err != nil || confirmed.IsError {
			t.Fatalf("confirmed MCP = %#v, %v", confirmed, err)
		}
	})
}

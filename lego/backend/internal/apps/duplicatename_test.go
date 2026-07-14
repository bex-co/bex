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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// duplicatename_test.go covers w4/m19's create-surface contract at the
// REST/GraphQL adapters (MCP's equivalent lives in internal/api alongside its
// session-based tool-calling harness): a same-workspace duplicate name is a
// clean 409/error, never a 500 and never a silent redeploy.

func TestREST_CreateDuplicateNameIs409(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","image":{"imagePath":"nginx"}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/services", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("POST /v1/services duplicate name: %d body=%s, want 409", rec.Code, rec.Body.String())
	}
	var out struct{ Error string }
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(out.Error, "already in use") {
		t.Errorf("error message = %q, want it to say the name is already in use", out.Error)
	}
}

func TestGraphQL_CreateServiceDuplicateNameErrors(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       context.Background(),
		RequestString: `mutation { createService(name: "web", image: "nginx") { id } }`,
	})
	if len(res.Errors) == 0 {
		t.Fatal("createService with a duplicate name: no error, want a conflict")
	}
	if !strings.Contains(strings.ToLower(res.Errors[0].Message), "already in use") {
		t.Errorf("error = %q, want it to say the name is already in use", res.Errors[0].Message)
	}
}

// TestCreateStoreConflictRaceIs409NotAn500 simulates the TOCTOU window
// between the pre-check and the actual write: nameTaken finds nothing (a
// concurrent create hasn't landed its CR yet), but the store's own
// UNIQUE(tenant_id, name) constraint catches it. That must still classify as
// ErrConflict, never bubble up as a bare/500 error.
func TestCreateStoreConflictRaceIs409NotAn500(t *testing.T) {
	rec := &recordingStore{err: fmt.Errorf("app: %w", store.ErrConflict)}
	svc, _ := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, rec)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})

	_, err := svc.create(ctx, CreateRequest{Name: "web", Image: "nginx:1"})
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("store-level race on create: got %v, want ErrConflict", err)
	}
}

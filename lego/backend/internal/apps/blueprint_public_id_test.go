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
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w2/m269: every surface shows a Blueprint by its exs- public id (the pinned
// Render CLI's IsBlueprintID refuses anything else), and the exs- id and its
// durable blp- twin address the same stored row on every input.
func TestBlueprintPublicIDOnEverySurface(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, onboardingManifest)
	durable := ids.New(ids.Blueprint)
	public := ids.BlueprintPublicID(durable)
	st.blueprints[durable] = store.Blueprint{ID: durable, TenantID: "tea-a", Name: "stack",
		Repo: "https://github.com/a/app", Branch: "main", Path: "render.yaml", Status: store.BlueprintStatusInSync}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rest := func(method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx))
		return rec
	}
	schema := blueprintSchema(t, svc)
	server := mcp.NewServer(&mcp.Implementation{Name: "m269", Version: "0"}, nil)
	svc.RegisterMCP(server)
	a, b := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, a, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "m269", Version: "0"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })

	// Every response names the public id and never leaks the durable key.
	answers := func(t *testing.T, surface, body string) {
		t.Helper()
		if !strings.Contains(body, public) || strings.Contains(body, durable) {
			t.Errorf("%s = %s, want %s and never %s", surface, body, public, durable)
		}
	}

	for _, spelling := range []string{public, durable} {
		t.Run(spelling[:3], func(t *testing.T) {
			rec := rest(http.MethodGet, "/v1/blueprints/"+spelling+"?ownerId=tea-a", "")
			if rec.Code != http.StatusOK {
				t.Fatalf("REST GET = %d %s", rec.Code, rec.Body)
			}
			answers(t, "REST GET", rec.Body.String())
			if rec := rest(http.MethodGet, "/v1/blueprints/"+spelling+"/syncs?ownerId=tea-a", ""); rec.Code != http.StatusOK {
				t.Errorf("REST GET syncs = %d %s", rec.Code, rec.Body)
			}
			rec = rest(http.MethodPatch, "/v1/blueprints/"+spelling, `{"ownerId":"tea-a","autoSync":true}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("REST PATCH = %d %s", rec.Code, rec.Body)
			}
			answers(t, "REST PATCH", rec.Body.String())

			out := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: fmt.Sprintf(`{ blueprint(id:%q, ownerId:"tea-a") { id } blueprintSyncs(id:%q, ownerId:"tea-a") { id } }`, spelling, spelling)})
			if len(out.Errors) > 0 {
				t.Fatalf("GraphQL: %v", out.Errors)
			}
			raw, _ := json.Marshal(out.Data)
			answers(t, "GraphQL blueprint", string(raw))

			res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_blueprint", Arguments: map[string]any{"id": spelling}})
			if err != nil || res.IsError {
				t.Fatalf("MCP get_blueprint = %+v, %v", res, err)
			}
			raw, _ = json.Marshal(res.StructuredContent)
			answers(t, "MCP get_blueprint", string(raw))
		})
	}

	t.Run("lists", func(t *testing.T) {
		rec := rest(http.MethodGet, "/v1/blueprints?ownerId=tea-a", "")
		answers(t, "REST list", rec.Body.String())
		var rows []struct {
			Cursor string `json:"cursor"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil || len(rows) != 1 || rows[0].Cursor != public {
			t.Fatalf("REST list rows = %+v (%v), want one row whose cursor is %s", rows, err, public)
		}
		// A cursor saved before the alias pages past the same row.
		for _, cursor := range []string{public, durable} {
			if rec := rest(http.MethodGet, "/v1/blueprints?ownerId=tea-a&cursor="+cursor, ""); strings.TrimSpace(rec.Body.String()) != "[]" {
				t.Errorf("REST list after %s = %s, want the page past the only row", cursor, rec.Body)
			}
		}
		out := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ blueprints(ownerId:"tea-a") { id } }`})
		if len(out.Errors) > 0 {
			t.Fatalf("GraphQL: %v", out.Errors)
		}
		raw, _ := json.Marshal(out.Data)
		answers(t, "GraphQL blueprints", string(raw))
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_blueprints", Arguments: map[string]any{}})
		if err != nil || res.IsError {
			t.Fatalf("MCP list_blueprints = %+v, %v", res, err)
		}
		raw, _ = json.Marshal(res.StructuredContent)
		answers(t, "MCP list_blueprints", string(raw))
	})

	t.Run("an unknown exs- id is not found", func(t *testing.T) {
		if rec := rest(http.MethodGet, "/v1/blueprints/"+ids.BlueprintPublicID(ids.New(ids.Blueprint))+"?ownerId=tea-a", ""); rec.Code != http.StatusNotFound {
			t.Errorf("GET unknown exs- = %d %s, want 404", rec.Code, rec.Body)
		}
	})

	t.Run("disconnect by the durable key", func(t *testing.T) {
		if rec := rest(http.MethodDelete, "/v1/blueprints/"+durable+"?ownerId=tea-a", ""); rec.Code != http.StatusNoContent {
			t.Fatalf("DELETE by blp- = %d %s", rec.Code, rec.Body)
		}
		if rec := rest(http.MethodGet, "/v1/blueprints/"+public+"?ownerId=tea-a", ""); rec.Code != http.StatusNotFound {
			t.Errorf("GET by exs- after the blp- disconnect = %d, want 404", rec.Code)
		}
	})
}

// The takeover phrase names the public id, and a phrase naming either
// spelling confirms the same stored owner (w2/m269).
func TestBlueprintTakeoverPhraseAcceptsBothSpellings(t *testing.T) {
	durable := ids.New(ids.Blueprint)
	public := ids.BlueprintPublicID(durable)
	if got := BlueprintTakeoverConfirmation(durable); got != "takeover blueprint "+public {
		t.Fatalf("phrase = %q, want the exs- id", got)
	}
	for _, phrase := range []string{"takeover blueprint " + public, "takeover blueprint " + durable} {
		if got := takeoverExpectedOwner(phrase); got != durable {
			t.Errorf("takeoverExpectedOwner(%q) = %q, want the stored %s", phrase, got, durable)
		}
	}
	err := blueprintOwnershipError(blueprintOwnershipConflict{kind: store.BlueprintClaimService, name: "api", owner: durable})
	if msg := err.Error(); !strings.Contains(msg, public) || strings.Contains(msg, durable) {
		t.Errorf("ownership refusal = %q, want the exs- id only", msg)
	}
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Params["owningBlueprintId"] != public {
		t.Errorf("ownership refusal details = %+v, want owningBlueprintId %s", coded, public)
	}
}

// A service managed by a Blueprint reports the public id; the label keeps
// the durable key.
func TestAppViewBlueprintIDIsPublic(t *testing.T) {
	durable := ids.New(ids.Blueprint)
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "api", Labels: map[string]string{core.LabelBlueprint: durable}}}
	if got := view(app).BlueprintID; got != ids.BlueprintPublicID(durable) {
		t.Fatalf("AppView.BlueprintID = %q, want %q", got, ids.BlueprintPublicID(durable))
	}
	if app.Labels[core.LabelBlueprint] != durable {
		t.Fatal("projecting the view rewrote the durable label")
	}
}

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

package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestCreatingUnderAJustDeletedNameSucceedsOnEverySurface (w2/075, w2/m365):
// a service create under the name of one just deleted, whose App the
// operator's finalizer still holds, succeeds at once on REST, GraphQL and MCP,
// as on Render. The new service gets its own App object beside the old one.
func TestCreatingUnderAJustDeletedNameSucceedsOnEverySurface(t *testing.T) {
	setup := func(t *testing.T) (http.Handler, *Server, client.Client) {
		t.Helper()
		gone := sampleApp("gone")
		gone.Finalizers = []string{"app.bex.co/finalizer"}

		cl := fakeClient(gone)
		if err := cl.Delete(context.Background(), gone); err != nil {
			t.Fatal(err)
		}
		h, srv := serverWith(t, &core.Base{Client: cl, Namespace: "default", Authz: &fakeChecker{allow: true}}, Deps{})
		return h, srv, cl
	}
	successor := func(t *testing.T, cl client.Client) {
		t.Helper()
		var list appv1alpha1.AppList
		if err := cl.List(context.Background(), &list, client.InNamespace("default")); err != nil {
			t.Fatal(err)
		}
		live := 0
		for _, a := range list.Items {
			if a.DeletionTimestamp.IsZero() {
				live++
				if !strings.HasPrefix(a.Name, "gone-") {
					t.Fatalf("the new service's App is %q, want gone-xxxx beside the old one", a.Name)
				}
			}
		}
		if live != 1 || len(list.Items) != 2 {
			t.Fatalf("%d Apps (%d live), want the old one tearing down and one new", len(list.Items), live)
		}
	}
	t.Run("REST", func(t *testing.T) {
		h, _, cl := setup(t)
		res := do(t, h, http.MethodPost, "/v1/services", testToken, `{"name":"gone","ownerId":"default","image":{"ownerId":"default","imagePath":"nginx"}}`)
		if res.Code != http.StatusCreated {
			t.Fatalf("POST /v1/services = %d %s, want 201", res.Code, res.Body)
		}
		successor(t, cl)
	})
	t.Run("GraphQL", func(t *testing.T) {
		h, _, cl := setup(t)
		if _, errs, body := rawGraphQL(t, h, `mutation { createService(name: "gone", image: "nginx") { id } }`); len(errs) != 0 {
			t.Fatalf("createService = %s, want success", body)
		}
		successor(t, cl)
	})
	t.Run("MCP", func(t *testing.T) {
		_, srv, cl := setup(t)
		cs := mcpSessionAs(t, srv, "dana")
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "create_web_service", Arguments: map[string]any{"name": "gone", "image": "nginx", "runtime": "image"}})
		if err != nil || res.IsError {
			t.Fatalf("create_web_service = %s, %v; want success", fmtMCP(res), err)
		}
		successor(t, cl)
	})
}

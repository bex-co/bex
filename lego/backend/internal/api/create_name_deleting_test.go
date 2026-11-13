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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestCreatingUnderAStillDeletingNameIsCodedOnEverySurface (w2/075): a
// service create under the name of one just deleted, whose App the
// operator's finalizer still holds, answers one named 409 with a retry hint on
// REST, GraphQL and MCP — never a 500, never the live holder's "already in
// use".
func TestCreatingUnderAStillDeletingNameIsCodedOnEverySurface(t *testing.T) {
	want := codedRefusal{
		status: http.StatusConflict, code: apps.ServiceNameStillDeletingCode,
		msg:    `a service named "gone" was just deleted and its cleanup is still finishing; retry in a minute to create a service with this name`,
		params: map[string]any{"service": "gone"},
	}
	setup := func(t *testing.T) (http.Handler, *Server) {
		t.Helper()
		gone := sampleApp("gone")
		gone.Finalizers = []string{"app.bex.co/finalizer"}
		adm := &admission{}
		cl := adm.client(gone)
		if err := cl.Delete(context.Background(), gone); err != nil {
			t.Fatal(err)
		}
		h, srv := serverWith(t, &core.Base{Client: cl, Namespace: "default", Authz: &fakeChecker{allow: true}}, Deps{})
		// A teardown that outlasts the create's wait (w2/m361), kept short here.
		srv.Apps.NameReleaseWait = 100 * time.Millisecond
		return h, srv
	}
	t.Run("REST", func(t *testing.T) {
		h, _ := setup(t)
		want.onREST(t, h, http.MethodPost, "/v1/services", `{"name":"gone","ownerId":"default","image":{"ownerId":"default","imagePath":"nginx"}}`)
	})
	t.Run("GraphQL", func(t *testing.T) {
		h, _ := setup(t)
		want.onGraphQL(t, h, `mutation { createService(name: "gone", image: "nginx") { id } }`)
	})
	t.Run("MCP", func(t *testing.T) {
		_, srv := setup(t)
		want.onMCP(t, mcpSessionAs(t, srv, "dana"), "create_web_service", map[string]any{"name": "gone", "image": "nginx", "runtime": "image"})
	})
}

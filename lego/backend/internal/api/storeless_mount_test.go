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
	"net/http"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestStorelessUsageAnswersUnavailable (w5/113): without the control-plane
// store, usage answers an uncoded 503 on every surface. The audit log's twin is
// TestAuditReadSurfaceAuthMatrix's store-less case.
func TestStorelessUsageAnswersUnavailable(t *testing.T) {
	h, srv := serverWith(t, &core.Base{Namespace: "default"}, Deps{})
	unavailable := codedRefusal{status: http.StatusServiceUnavailable, msg: "usage unavailable"}
	unavailable.onREST(t, h, http.MethodGet, "/v1/usage", "")
	unavailable.onGraphQL(t, h, `{ usage { period } }`)
	unavailable.onMCP(t, mcpSessionAs(t, srv, "dana"), "get_usage", map[string]any{})
}

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
	"net/http"
	"os"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
)

// validated serves mux with every response to a Render-shaped route checked
// against the pinned Render schema, through the same validator and allowlist
// the composed internal/api suites use (rendertest.Wrap): a service-handler
// change that drifts a field fails the test that exercised it. The httptest
// suites here drive the bare feature mux, without the production request gate,
// so routes are matched by rendertest's MuxMatcher.
func validated(t testing.TB, mux *http.ServeMux) http.Handler {
	t.Helper()
	return rendertest.Wrap(t, mux, rendertest.Load(t).MuxMatcher(mux))
}

// TestMain fails a full run that leaves an internal/apps Allowlist entry
// unexercised (rendertest.RunSuite).
func TestMain(m *testing.M) {
	os.Exit(rendertest.RunSuite(m, rendertest.SuiteApps))
}

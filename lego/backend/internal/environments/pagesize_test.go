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

package environments

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestListEnvironmentsPagesAtTheAdvertisedDefault pins the page SIZE of
// list_environments, which nothing checked.
//
// The tool passed core.PageLimit(in.Limit) straight through, and PageLimit(0)
// is 1 — correct for a query string, where PageParams substitutes the default
// before clamping, but wrong for a typed MCP int where 0 IS "omitted". So a
// caller that named a cursor and omitted the limit got ONE environment per
// page while the tool's own jsonschema advertised "1-100, default 20".
//
// It is the kind of defect that survives a test suite: paging still terminated,
// every item still arrived, ordering was right, and only the number of round
// trips an agent had to make revealed it. The assertion is therefore on the
// page size itself, not on the eventual contents.
func TestListEnvironmentsPagesAtTheAdvertisedDefault(t *testing.T) {
	ctx := ctxAs("user-a")
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	// More environments than the default page, so "a full default page" and
	// "everything there is" are distinguishable answers.
	total := core.DefaultPageLimit + 7
	for i := range total {
		if _, err := st.CreateEnvironment(ctx, "prj-1", "tea-a", fmt.Sprintf("env-%03d", i)); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	svc, _ := newServiceWithClient(st)
	client := newMCPClient(t, ctx, svc)

	call := func(t *testing.T, args map[string]any) []EnvironmentView {
		t.Helper()
		res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "list_environments", Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("list_environments(%v): %v isErr=%v", args, err, res.IsError)
		}
		var out environmentsResult
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return out.Environments
	}

	// An unpaged call returns the store's own (map) order untouched, so its
	// first element is arbitrary. A page is cursor-SORTED, so the cursor to
	// resume from is the smallest id — taking all[0] here would make the
	// expected page size depend on where a random id happened to sort.
	all := call(t, map[string]any{"projectId": "prj-1"})
	if len(all) != total {
		t.Fatalf("unpaged = %d environments, want all %d", len(all), total)
	}
	ids := make([]string, 0, len(all))
	for _, e := range all {
		ids = append(ids, e.ID)
	}
	slices.Sort(ids)
	firstCursor := ids[0]

	t.Run("a cursor with no limit yields the default page, not one item", func(t *testing.T) {
		got := len(call(t, map[string]any{"projectId": "prj-1", "cursor": firstCursor}))
		if got != core.DefaultPageLimit {
			t.Fatalf("page = %d environments, want DefaultPageLimit (%d); 1 means the "+
				"omitted limit was clamped up from zero instead of defaulted", got, core.DefaultPageLimit)
		}
	})

	t.Run("an explicit limit is honored", func(t *testing.T) {
		if got := len(call(t, map[string]any{"projectId": "prj-1", "limit": 5})); got != 5 {
			t.Fatalf("page = %d environments, want 5", got)
		}
	})
}

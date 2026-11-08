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

package store

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// w5/m287: names are stored per workspace, not unique, bounded to Render's 64
// characters, and pruned only once a sandbox is gone and the row is old.
func TestSandboxNamesPG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sandbox-names").ID
	other := lifecycleTenant(t, st, "sandbox-names-other").ID
	stamp := time.Now().UnixNano()
	a, b, c := fmt.Sprintf("sbx-a%d", stamp), fmt.Sprintf("sbx-b%d", stamp), fmt.Sprintf("sbx-c%d", stamp)
	for _, id := range []string{a, b} {
		if err := st.SetSandboxName(ctx, ws, id, "my agent ✨"); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SetSandboxName(ctx, other, c, "elsewhere"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSandboxName(ctx, ws, c, strings.Repeat("a", 65)); err == nil {
		t.Error("a 65-character name was stored")
	}
	names, err := st.SandboxNames(ctx, ws)
	if err != nil || len(names) != 2 || names[a] != "my agent ✨" || names[b] != "my agent ✨" {
		t.Fatalf("names = %v (err %v), want both same-named sandboxes and nothing from another workspace", names, err)
	}
	// Fresh rows survive a prune even when the sandbox is not listed yet.
	if err := st.PruneSandboxNames(ctx, ws, []string{a}); err != nil {
		t.Fatal(err)
	}
	if names, _ = st.SandboxNames(ctx, ws); len(names) != 2 {
		t.Fatalf("a young row was pruned: %v", names)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE sandbox_names SET created_at = now() - interval '1 hour' WHERE workspace_id = $1`, ws); err != nil {
		t.Fatal(err)
	}
	if err := st.PruneSandboxNames(ctx, ws, []string{a}); err != nil {
		t.Fatal(err)
	}
	if names, _ = st.SandboxNames(ctx, ws); len(names) != 1 || names[a] == "" {
		t.Fatalf("after prune = %v, want only the live sandbox's name", names)
	}
	if err := st.DeleteSandboxName(ctx, ws, a); err != nil {
		t.Fatal(err)
	}
	if names, _ = st.SandboxNames(ctx, ws); len(names) != 0 {
		t.Fatalf("after delete = %v", names)
	}
}

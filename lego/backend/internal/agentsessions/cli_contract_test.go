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

package agentsessions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestViewMatchesCLIEditContract pins the session fields `bex edit ags-…`
// reads (lego/cli/internal/desktop): the CLI imports no sibling module, so it
// decodes them from lego/cli/testdata/agent-session-ssh-contract.json, and a
// rename here must move that golden with it.
func TestViewMatchesCLIEditContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "cli", "testdata", "agent-session-ssh-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]any
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(View{ID: "ags-x", SSHAddress: "ags-x@ssh.bex.co"})
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err := json.Unmarshal(encoded, &view); err != nil {
		t.Fatal(err)
	}
	for key := range golden {
		if _, ok := view[key]; !ok {
			t.Errorf("View no longer emits %q, which bex edit reads", key)
		}
	}
}

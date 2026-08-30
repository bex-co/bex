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

package sandbox

import (
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
)

// The hibernation snapshot must carry every reviewed agent profile's on-disk
// session state — it is the substrate of ADR047 D3's `session/load` resume
// rung (ADR059 D3 continuity amendment, w5/m84). The manifest declares the
// dirs (`sessionState`); this guard fails when a profile declares state the
// hibernate script would silently omit — the omission that made every resume
// a cold start. The driver's /var/run/bex-agent status dir (the persisted ACP
// session identity adoptPersistedSession reads) rides the same contract.
func TestHibernateScriptCarriesEveryProfileSessionStateDir(t *testing.T) {
	ids := agentsession.AgentProfileIDs()
	if len(ids) == 0 {
		t.Fatal("no agent profiles loaded")
	}
	declared := 0
	for _, id := range ids {
		profile, ok := agentsession.LookupAgentProfile(id)
		if !ok {
			t.Fatalf("profile %q not loadable", id)
		}
		for _, entry := range profile.SessionState {
			declared++
			// Whole-token match: `.claude` must not be satisfied by the
			// presence of `.claude.json` — dropping the session DIR while
			// keeping the config file is exactly the cold-start regression
			// this guard exists for.
			if !strings.Contains(hibernateScript, " "+entry+" ") &&
				!strings.Contains(hibernateScript, " "+entry+";") {
				t.Errorf("profile %q session-state entry %q is not tarred by hibernateScript", profile.ID, entry)
			}
		}
	}
	if declared == 0 {
		t.Fatal("no profile declares sessionState — the session/load rung has no substrate")
	}
	if !strings.Contains(hibernateScript, "/var/run/bex-agent") {
		t.Error("hibernateScript omits /var/run/bex-agent — the persisted ACP session identity would not survive hibernation")
	}
}

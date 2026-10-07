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

package store_test

import (
	"os"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

// TestTheFailureCodeBackfillWritesTheCapacityCode (w5/m132): migration 0144
// codes old capacity failures with the code bex-api records today, so renaming
// sandbox.CodeSandboxCapacityLimit cannot silently orphan the backfilled rows.
// The sentence the backfill matches is not pinned: it is the wording stored
// then, whatever CapacityFailureReason later says.
func TestTheFailureCodeBackfillWritesTheCapacityCode(t *testing.T) {
	up, err := os.ReadFile("migrations/0144_agent_session_failure_reason_code.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(up), "failure_reason_code = '"+sandbox.CodeSandboxCapacityLimit+"'") {
		t.Errorf("migration 0144 does not backfill %s", sandbox.CodeSandboxCapacityLimit)
	}
}

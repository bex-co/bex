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
	"errors"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/m544: Render picks a new service's default Python by when it was
// created, so a session's driver learns when the session was created, and a
// snapshot build's when the build was, in RFC 3339 and UTC whatever zone the
// row was read in.
func TestDriverEnvCarriesTheSessionCreationTime(t *testing.T) {
	created := time.Date(2026, time.November, 20, 10, 30, 0, 0, time.FixedZone("CET", 3600))
	for _, record := range []store.AgentSession{
		{ID: "ags-x", WorkspaceID: "tea-a", Repo: "acme/widgets", Branch: "bex-agent/ags-x", Turns: 1, CreatedAt: created},
		{ID: "ags-y", WorkspaceID: "tea-a", Turns: 3, CreatedAt: created}, // a repo-less session, on a later turn
	} {
		env := (&Service{}).driverEnv(AgentConfig{Agent: "claude"}, record, store.AgentEnvironment{})
		if got := env[envCreatedAt]; got != "2026-11-20T09:30:00Z" {
			t.Errorf("session %s: %s = %q, want 2026-11-20T09:30:00Z", record.ID, envCreatedAt, got)
		}
	}
	// An unknown creation time is no value at all, which the driver reads as
	// unknown (the legacy default), never as the zero date.
	env := (&Service{}).driverEnv(AgentConfig{Agent: "claude"}, store.AgentSession{ID: "ags-z", WorkspaceID: "tea-a", Turns: 1}, store.AgentEnvironment{})
	if got, ok := env[envCreatedAt]; ok {
		t.Errorf("an unknown creation time sets %s = %q", envCreatedAt, got)
	}
}

func TestBuilderEnvCarriesTheBuildCreationTime(t *testing.T) {
	created := time.Date(2026, time.November, 19, 23, 0, 0, 0, time.UTC)
	claimed := store.ClaimedAgentRepoEnvironmentBuild{Build: store.AgentRepoEnvironmentBuild{ID: "arb-x", WorkspaceID: "tea-a", Repo: "acme/api", CreatedAt: created}}
	env, err := (&Service{}).builderEnv(t.Context(), claimed, NetworkTierRegistries)
	if err != nil {
		t.Fatal(err)
	}
	if got := env[envCreatedAt]; got != "2026-11-19T23:00:00Z" {
		t.Fatalf("%s = %q, want the build's creation time", envCreatedAt, got)
	}
}

// The value is the platform's: a workspace or session secret cannot be
// named after it, so none can replace it in the sandbox.
func TestTheSessionCreationTimeIsNotASecretName(t *testing.T) {
	err := validateSecretName("name", envCreatedAt)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "AGENT_SECRET_NAME_RESERVED" {
		t.Fatalf("a secret named %s: %v, want AGENT_SECRET_NAME_RESERVED", envCreatedAt, err)
	}
}

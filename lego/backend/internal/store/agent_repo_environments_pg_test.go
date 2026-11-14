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
	"encoding/json"
	"errors"
	"testing"
)

// w9/m387, ADR109: environments upsert and list by repository, a new
// repository past the workspace's limit is refused while a replace at the
// limit is not, and delete reports a missing row.
func TestPGAgentRepoEnvironments(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "repo-envs")
	ctx := context.Background()
	setup := []byte(`[{"name":"deps","run":"npm ci"}]`)
	checks := []byte(`[{"name":"test","run":"npm test"}]`)
	for _, repo := range []string{"acme/web", "acme/api"} {
		if _, _, err := s.UpsertAgentRepoEnvironment(ctx, tenant.ID, repo, setup, checks, 2); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := s.UpsertAgentRepoEnvironment(ctx, tenant.ID, "acme/cli", setup, checks, 2); !errors.Is(err, ErrAgentRepoEnvironmentLimit) {
		t.Fatalf("past the limit = %v", err)
	}
	replaced, _, err := s.UpsertAgentRepoEnvironment(ctx, tenant.ID, "acme/api", []byte(`[]`), checks, 2)
	if err != nil || string(replaced.Setup) != "[]" {
		t.Fatalf("replace at the limit = %s / %v", replaced.Setup, err)
	}
	rows, err := s.ListAgentRepoEnvironments(ctx, tenant.ID)
	if err != nil || len(rows) != 2 || rows[0].Repo != "acme/api" || rows[1].Repo != "acme/web" {
		t.Fatalf("list = %+v / %v", rows, err)
	}
	got, err := s.GetAgentRepoEnvironment(ctx, tenant.ID, "acme/web")
	var steps []struct{ Name, Run string }
	if err != nil || json.Unmarshal(got.Checks, &steps) != nil || len(steps) != 1 || steps[0].Name != "test" || steps[0].Run != "npm test" {
		t.Fatalf("get = %s / %v", got.Checks, err)
	}
	if _, err := s.GetAgentRepoEnvironment(ctx, tenant.ID, "acme/none"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing get = %v", err)
	}
	if err := s.DeleteAgentRepoEnvironment(ctx, tenant.ID, "acme/web"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAgentRepoEnvironment(ctx, tenant.ID, "acme/web"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v", err)
	}
}

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
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// The workspace agent environment (w10/m17): no row until first written, a
// partial upsert keeps every field it does not name, the CHECKs bound the
// tier and host count, and the row goes with its workspace.
func TestAgentEnvironmentUpsertIsPartialAndBounded(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	tenant, err := st.CreateTenant(ctx, "agent-environment", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })

	if _, err := st.GetAgentEnvironment(ctx, tenant.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unwritten environment error = %v, want ErrNotFound", err)
	}
	model := "claude-sonnet-4-5"
	got, err := st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{DefaultModel: &model})
	if err != nil {
		t.Fatal(err)
	}
	if got.NetworkTier != "registries" || got.CustomHosts == nil || len(got.CustomHosts) != 0 || got.DefaultModel != model || got.UpdatedAt.IsZero() {
		t.Fatalf("first partial write = %+v, want the column defaults plus the model", got)
	}
	tier, hosts := "custom", []string{"a.example.com", "b.example.com"}
	if got, err = st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{NetworkTier: &tier, CustomHosts: &hosts}); err != nil {
		t.Fatal(err)
	}
	if got.NetworkTier != tier || !slices.Equal(got.CustomHosts, hosts) || got.DefaultModel != model {
		t.Fatalf("second partial write = %+v, lost the model or the hosts", got)
	}
	read, err := st.GetAgentEnvironment(ctx, tenant.ID)
	if err != nil || read.NetworkTier != tier || !slices.Equal(read.CustomHosts, hosts) || read.DefaultModel != model {
		t.Fatalf("read back = %+v err=%v", read, err)
	}
	empty := []string{}
	if got, err = st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{CustomHosts: &empty}); err != nil || len(got.CustomHosts) != 0 || got.NetworkTier != tier {
		t.Fatalf("clearing the hosts = %+v err=%v", got, err)
	}

	bad := "internet"
	if _, err := st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{NetworkTier: &bad}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown tier error = %v, want ErrInvalid", err)
	}
	tooMany := make([]string, 33)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("h%d.example.com", i)
	}
	if _, err := st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{CustomHosts: &tooMany}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("33 hosts error = %v, want ErrInvalid", err)
	}
	if _, err := st.UpsertAgentEnvironment(ctx, "tea-00000000000000000000", AgentEnvironmentPatch{NetworkTier: &tier}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown workspace error = %v, want ErrNotFound", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetAgentEnvironment(ctx, tenant.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("environment survived its workspace: %v", err)
	}
}

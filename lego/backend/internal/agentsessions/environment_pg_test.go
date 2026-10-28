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
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// The workspace agent environment against the real control-plane store:
// defaults, round-trip, admin-only writes and the typed validation errors on
// REST, GraphQL and MCP (w10/m17 t003).
func TestEnvironmentPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pg := store.NewPGStore(pool)
	created := 0
	tenant := func(name string) string {
		t.Helper()
		created++
		ws, err := pg.CreateTenant(ctx, fmt.Sprintf("%s-%d", name, created), store.PlanPro)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = pg.DeleteTenant(ctx, ws.ID) })
		return ws.ID
	}

	t.Run("round trip", func(t *testing.T) {
		ws := tenant("agent-environment-roundtrip")
		exerciseEnvironmentRoundTrip(t, environmentService(ws, pg), ws)
	})
	t.Run("authorization", func(t *testing.T) {
		ws, other := tenant("agent-environment-authz"), tenant("agent-environment-other")
		exerciseEnvironmentAuthorization(t, environmentService(ws, pg), ws, other)
	})
	t.Run("validation", func(t *testing.T) {
		exerciseEnvironmentValidation(t, func() (*Service, string) {
			ws := tenant("agent-environment-validation")
			return environmentService(ws, pg), ws
		})
	})
}

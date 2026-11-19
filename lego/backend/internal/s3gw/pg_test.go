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

package s3gw

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/fakes3"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// auditLog records the rows the Base writes.
type auditLog struct {
	mu   sync.Mutex
	rows []core.AuditEvent
}

func (a *auditLog) Record(_ context.Context, ev core.AuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rows = append(a.rows, ev)
	return nil
}

func (a *auditLog) byVerb(verb string) []core.AuditEvent {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []core.AuditEvent
	for _, r := range a.rows {
		if r.Verb == verb {
			out = append(out, r)
		}
	}
	return out
}

// pgRig is the S3 surface over real Postgres (buckets, usage, the multipart rows) and
// a fake Wasabi per region, with access keys and OpenBao in memory.
type pgRig struct {
	*rig
	PG    *store.PGStore
	Pool  *pgxpool.Pool
	Svc   *objectstorage.Service
	Fakes map[objectstorage.Region]*fakes3.Server
	Audit *auditLog
}

func newPGRig(t *testing.T) *pgRig {
	t.Helper()
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
	t.Cleanup(pool.Close)
	pg := store.NewPGStore(pool)
	tenant, err := pg.CreateTenant(ctx, "s3gw-"+time.Now().Format("150405.000000000"), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		for _, q := range []string{
			`DELETE FROM object_storage_usage WHERE workspace_id=$1`, `DELETE FROM object_buckets WHERE workspace_id=$1`,
			`DELETE FROM object_access_keys WHERE workspace_id=$1`, `DELETE FROM object_multipart_uploads WHERE workspace_id=$1`,
			`DELETE FROM object_storage_writes WHERE workspace_id=$1`, `DELETE FROM tenants WHERE id=$1`,
		} {
			_, _ = pool.Exec(bg, q, tenant.ID)
		}
	})
	g := newRig(t)
	useRealHandlers(t)
	g.ws = tenant.ID
	g.authz.workspace = tenant.ID
	p := &pgRig{rig: g, PG: pg, Pool: pool, Fakes: map[objectstorage.Region]*fakes3.Server{}, Audit: &auditLog{}}
	stores := map[objectstorage.Region]objectstorage.Store{}
	for _, region := range objectstorage.Regions {
		fake := fakes3.New(region.Bucket())
		t.Cleanup(fake.Close)
		p.Fakes[region] = fake
		stores[region] = objectstorage.NewStore(objectstorage.Config{
			Endpoint: fake.URL, Region: region.WasabiRegion(), Bucket: region.Bucket(), AccessKey: "a", SecretKey: "s", MaxAttempts: 1,
		})
	}
	p.Svc = objectstorage.NewService(pg, stores)
	p.gw.cfg.Service = p.Svc
	p.gw.cfg.Base = &core.Base{Authz: g.authz, Audit: p.Audit}
	p.gw.wireAudit()
	return p
}

// bucketNames reads a ListBuckets answer's names.
func bucketNames(t *testing.T, body string) []string {
	t.Helper()
	var names []string
	for _, part := range strings.Split(body, "<Name>")[1:] {
		names = append(names, strings.SplitN(part, "</Name>", 2)[0])
	}
	return names
}

// otherWorkspace is a workspace id no key belongs to, whose bucket rows the test
// removes.
func (p *pgRig) otherWorkspace() string {
	id := ids.New(ids.Workspace)
	p.t.Cleanup(func() {
		_, _ = p.Pool.Exec(context.Background(), `DELETE FROM object_multipart_uploads WHERE workspace_id=$1`, id)
		_, _ = p.Pool.Exec(context.Background(), `DELETE FROM object_buckets WHERE workspace_id=$1`, id)
	})
	return id
}

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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func TestPGProductAnalyticsLifecyclePrivacyAndSampling(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	subject := ids.New(ids.Owner)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM account_deletions WHERE subject=$1", subject)
	}()
	app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "product-test", Image: "nginx", Type: "static_site", Tier: "free", Port: 80, Replicas: 1})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Second)
	event := core.ProductActivity{WorkspaceID: tenant.ID, ResourceID: app.ID, ResourceType: "static_site", EventType: "created", At: at, ActorID: subject, ActorType: "human"}
	for range 2 {
		if err := st.RecordProductActivity(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM product_activity_events WHERE source_key=$1", "created:"+app.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("dedup=%d err=%v", count, err)
	}
	for _, sample := range []struct {
		delta      time.Duration
		live, want bool
	}{
		{0, true, true}, {time.Minute, true, false}, {2 * time.Minute, false, true}, {time.Minute, true, false},
	} {
		changed, err := st.RecordProductHosting(ctx, tenant.ID, app.ID, "static_site", sample.live, at.Add(sample.delta))
		if err != nil || changed != sample.want {
			t.Fatalf("sample %+v changed=%v err=%v", sample, changed, err)
		}
	}
	var live, wasLive bool
	if err := pool.QueryRow(ctx, "SELECT live,was_live FROM product_hosting_daily WHERE resource_id=$1 ORDER BY day DESC LIMIT 1", app.ID).Scan(&live, &wasLive); err != nil || live || !wasLive {
		t.Fatalf("latest=%v was=%v err=%v", live, wasLive, err)
	}
	domain, err := st.CreateDomain(ctx, app.ID, "product-"+tenant.ID+".test", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RecordProductDomainTLS(ctx, domain.ID, true, at); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordProductDomainTLS(ctx, domain.ID, false, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var first time.Time
	if err := pool.QueryRow(ctx, "SELECT tls_ready,first_tls_ready_at FROM product_domain_observations WHERE domain_id=$1", domain.ID).Scan(&live, &first); err != nil || live || !first.Equal(at) {
		t.Fatalf("TLS ready=%v first=%s err=%v", live, first, err)
	}
	if err := st.CleanupAccountSubject(ctx, subject, "deleted-subject"); err != nil {
		t.Fatal(err)
	}
	var actor, kind string
	if err := pool.QueryRow(ctx, "SELECT actor_id,actor_type FROM product_activity_events WHERE source_key=$1", "created:"+app.ID).Scan(&actor, &kind); err != nil || actor != "" || kind != "unknown" {
		t.Fatalf("privacy=%q/%q err=%v", actor, kind, err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO account_deletions(subject,deleted_marker) VALUES ($1,'deleted-subject')", subject); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordProductActivity(ctx, event); err == nil {
		t.Fatal("recorder resurrected a deleting subject")
	}
	// Expiring a creation must not make subsequent real deletion erase recent history.
	if _, err := pool.Exec(ctx, "UPDATE product_activity_events SET at=$2 WHERE source_key=$1", "created:"+app.ID, at.AddDate(0, 0, -100)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PurgeProductAnalytics(ctx, at.AddDate(0, 0, -90)); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteApp(ctx, app.ID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM product_activity_events WHERE parent_id=$1", app.ID).Scan(&count); err != nil || count < 1 {
		t.Fatalf("recent history lost: %d %v", count, err)
	}
}

func TestPGProductRollbackRemovesOnlyProvisionalAppAndFacts(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	apps := make([]App, 0, 2)
	for _, name := range []string{"rollback", "unrelated"} {
		app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: name, Image: "nginx", Type: "web_service", Tier: "free", Port: 80, Replicas: 1})
		if err != nil {
			t.Fatal(err)
		}
		apps = append(apps, app)
		// CreateApp also persists the initial deploy and its parent-scoped fact.
		if err := st.RecordProductActivity(ctx, core.ProductActivity{
			WorkspaceID: tenant.ID, ResourceID: app.ID, ResourceType: "web_service",
			EventType: "created", At: time.Now(), ActorType: "unknown",
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO product_inventory_lifecycle
			(resource_id,workspace_id,source,resource_type,created_at,first_seen_at,last_seen_at,state)
			VALUES($1,$2,'services','web_service',now(),now(),now(),'unknown')`, app.ID, tenant.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.RollbackAppCreation(ctx, apps[0].ID); err != nil {
		t.Fatal(err)
	}
	for index, app := range apps {
		for _, query := range []string{
			"SELECT count(*) FROM apps WHERE id=$1",
			"SELECT count(*) FROM deploys WHERE app_id=$1",
			"SELECT count(*) FROM product_activity_events WHERE resource_id=$1",
			"SELECT count(*) FROM product_activity_events WHERE parent_id=$1",
			"SELECT count(*) FROM product_inventory_lifecycle WHERE resource_id=$1",
		} {
			var count int
			if err := pool.QueryRow(ctx, query, app.ID).Scan(&count); err != nil || count != index {
				t.Fatalf("%s: count=%d want=%d err=%v", query, count, index, err)
			}
		}
	}
}

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

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The lifecycle rules of w34/m543 on a real PostgreSQL (17 in CI).

// upgradeSurfaces sends one version upgrade of d through REST, GraphQL and
// MCP. dryRun previews on REST and MCP (GraphQL's updateDatabaseVersion has
// none, so it writes). Each answer is its error code, or "" and the notice.
type upgradeAnswer struct{ code, notice string }

func upgradeThroughEverySurface(t *testing.T, svc *Service, d, target string, dryRun bool) map[string]upgradeAnswer {
	t.Helper()
	ctx := context.Background()
	out := map[string]upgradeAnswer{}

	rec := serveREST(svc, "PATCH", "/v1/postgres/"+d, fmt.Sprintf(`{"version":%q,"dryRun":%v}`, target, dryRun))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	code, _ := body["code"].(string)
	notice, _ := body["upgradeNotice"].(string)
	if (rec.Code == 200) == (code != "") {
		t.Fatalf("REST PATCH = %d %s", rec.Code, rec.Body.String())
	}
	out["rest"] = upgradeAnswer{code, notice}

	cs, closeMCP := pgMCPSession(t, svc)
	defer closeMCP()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_postgres", Arguments: map[string]any{"postgresId": d, "version": target, "dryRun": dryRun}})
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			texts = append(texts, text.Text)
		}
	}
	answer := upgradeAnswer{}
	if res.IsError {
		for _, c := range []string{PostgresUpgradeLogicalSlotsBehindCode, PostgresUpgradeLogicalSlotsUnreadableCode} {
			if strings.Contains(strings.Join(texts, " "), c) {
				answer.code = c
			}
		}
		if answer.code == "" {
			t.Fatalf("MCP update_postgres error = %q", texts)
		}
	} else if len(texts) == 2 {
		// The view, then the warning beside it.
		answer.notice = texts[1]
	}
	out["mcp"] = answer

	schema, err := pgGQLSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	gql := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: fmt.Sprintf(`mutation { updateDatabaseVersion(id:%q, version:%q) { id upgradeNotice } }`, d, target)})
	answer = upgradeAnswer{}
	if len(gql.Errors) > 0 {
		answer.code, _ = gql.Errors[0].Extensions["code"].(string)
	} else if m, ok := gql.Data.(map[string]any)["updateDatabaseVersion"].(map[string]any); ok {
		answer.notice, _ = m["upgradeNotice"].(string)
	}
	out["graphql"] = answer
	return out
}

// publishAt makes f's database an enabled publisher observed at version,
// ready to upgrade: its reader is the publisher (REPLICATION, as the
// operator gives it) and a completed backup of its generation exists.
func (f *lrFixture) publishAt(version string) {
	f.t.Helper()
	ctx := context.Background()
	var d appv1alpha1.Database
	if err := f.svc.Client.Get(ctx, client.ObjectKeyFromObject(f.db), &d); err != nil {
		f.t.Fatal(err)
	}
	d.Spec.Version, d.Status.CurrentVersion, d.Status.BackupsEnabled = version, version, true
	d.Spec.LogicalReplication = &appv1alpha1.DatabaseLogicalReplication{PublisherRole: f.readerRole, Schemas: []string{"public"}}
	if err := f.svc.Client.Update(ctx, &d); err != nil {
		f.t.Fatal(err)
	}
	backup := completedVersionBackup(d.Name, d.Name)
	if err := f.svc.Client.Get(ctx, client.ObjectKeyFromObject(backup), backup.DeepCopy()); err != nil {
		if err := f.svc.Client.Create(ctx, backup); err != nil {
			f.t.Fatal(err)
		}
	}
	execAs(f.t, f.superuser, fmt.Sprintf(`ALTER ROLE %s REPLICATION`, f.readerRole))
}

func (f *lrFixture) specVersion() string {
	f.t.Helper()
	var d appv1alpha1.Database
	if err := f.svc.Client.Get(context.Background(), client.ObjectKeyFromObject(f.db), &d); err != nil {
		f.t.Fatal(err)
	}
	return d.Spec.Version
}

// A 17 publisher whose subscriber is paused while rows are written is
// refused on REST, GraphQL and MCP alike, by the slot's name, writing
// nothing: pg_upgrade would fail the whole upgrade on it. Once the
// subscriber streams again and confirms, every surface accepts, with no
// notice (DoD 4).
func TestUpgradeRefusedWhileALogicalSlotLags(t *testing.T) {
	f := newLRFixture(t)
	f.publishAt("17")
	old := slotCatchUpWait
	slotCatchUpWait = 15 * time.Second
	t.Cleanup(func() { slotCatchUpWait = old })

	slot := "lr_s_" + f.n
	var port string
	if conn, err := pgx.Connect(context.Background(), f.superuser); err == nil {
		_ = conn.QueryRow(context.Background(), `SHOW port`).Scan(&port)
		_ = conn.Close(context.Background())
	}
	execAs(t, f.owner, `CREATE PUBLICATION mine FOR TABLE public.orders`)
	execAs(t, f.reader, fmt.Sprintf(`SELECT pg_create_logical_replication_slot('%s', 'pgoutput')`, slot))
	execAs(t, f.subscriber, `CREATE TABLE public.orders (id int PRIMARY KEY, note text)`,
		fmt.Sprintf(`CREATE SUBSCRIPTION %s CONNECTION 'host=127.0.0.1 port=%s dbname=%s user=%s password=reader-pw' PUBLICATION mine WITH (create_slot = false, slot_name = '%s', copy_data = false, enabled = false)`,
			slot, port, f.dbname, f.readerRole, slot))
	execAs(t, f.owner, `INSERT INTO public.orders SELECT g, 'behind' FROM generate_series(1, 50) g`)

	for surface, got := range upgradeThroughEverySurface(t, f.svc, f.db.Name, "18", false) {
		if got.code != PostgresUpgradeLogicalSlotsBehindCode {
			t.Errorf("%s: a lagging slot answers %+v, want %s", surface, got, PostgresUpgradeLogicalSlotsBehindCode)
		}
	}
	rec := serveREST(f.svc, "PATCH", "/v1/postgres/"+f.db.Name, `{"version":"18"}`)
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), slot+" (no subscriber is connected to it)") {
		t.Errorf("REST refusal = %d %s, want 409 naming %s", rec.Code, rec.Body.String(), slot)
	}
	if v := f.specVersion(); v != "17" {
		t.Fatalf("a refused upgrade wrote spec.version %q", v)
	}

	// The subscriber streams again and catches up.
	execAs(t, f.subscriber, fmt.Sprintf(`ALTER SUBSCRIPTION %s ENABLE`, slot))
	execAs(t, f.owner, `INSERT INTO public.orders VALUES (51, 'streamed')`)
	waitForSQL(t, 30*time.Second, f.subscriber, `SELECT count(*)::text FROM public.orders`, "51")
	for surface, got := range upgradeThroughEverySurface(t, f.svc, f.db.Name, "18", true) {
		if got != (upgradeAnswer{}) {
			t.Errorf("%s: a caught-up slot answers %+v, want an accepted upgrade with no notice", surface, got)
		}
	}
	if v := f.specVersion(); v != "18" {
		t.Errorf("spec.version = %q after GraphQL's accepted upgrade, want 18", v)
	}
}

// From a source before 17, pg_upgrade drops every logical slot: the upgrade
// is accepted, and REST, GraphQL and MCP each answer the same notice naming
// the slot (DoD 4). The test server's own major is what CI runs; the
// observed major is what the rule reads, as the operator records it.
func TestUpgradeFromBefore17SaysTheSlotsDrop(t *testing.T) {
	f := newLRFixture(t)
	f.publishAt("16")
	slot := "lr_s_" + f.n
	execAs(t, f.reader, fmt.Sprintf(`SELECT pg_create_logical_replication_slot('%s', 'pgoutput')`, slot))

	answers := upgradeThroughEverySurface(t, f.svc, f.db.Name, "17", true)
	want := droppedSlotsNotice("16", []string{slot})
	for surface, got := range answers {
		if got.code != "" || got.notice != want {
			t.Errorf("%s: answers %+v, want the notice %q", surface, got, want)
		}
	}
	if v := f.specVersion(); v != "17" {
		t.Errorf("spec.version = %q, want 17: the upgrade is accepted", v)
	}
}

// A recovered copy's first subscriber enable is refused while its catalog
// holds subscriptions, which can only be its source's: the operator holds
// them still until the enable, and they would then compete with the source
// for its publisher's slot. Detached and dropped, the enable goes ahead.
func TestRecoveredCopyRefusesASubscriberWhileHoldingItsSourcesSubscriptions(t *testing.T) {
	f := newLRFixture(t)
	shortenLogicalReplicationWait(t, 300*time.Millisecond)
	ctx := context.Background()
	var d appv1alpha1.Database
	if err := f.svc.Client.Get(ctx, client.ObjectKeyFromObject(f.db), &d); err != nil {
		t.Fatal(err)
	}
	d.Spec.Recovery = &appv1alpha1.DatabaseRecovery{SourceDatabase: "dpg-source"}
	if err := f.svc.Client.Update(ctx, &d); err != nil {
		t.Fatal(err)
	}
	// What a restore carries: the source's subscription, in the copy's own
	// database, with the source's slot name.
	carried := "lr_s_" + f.n
	execAs(t, f.superuser, fmt.Sprintf(`CREATE SUBSCRIPTION %s CONNECTION 'host=127.0.0.1 dbname=nowhere' PUBLICATION mine WITH (connect = false)`, carried))
	t.Cleanup(func() {
		conn, err := pgx.Connect(context.Background(), f.superuser)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close(context.Background()) }()
		_, _ = conn.Exec(context.Background(), fmt.Sprintf(`ALTER SUBSCRIPTION %s SET (slot_name = NONE)`, carried))
		_, _ = conn.Exec(context.Background(), fmt.Sprintf(`DROP SUBSCRIPTION IF EXISTS %s`, carried))
	})
	enable := LogicalReplicationRequest{Database: f.db.Name, Enable: true, SubscriberRole: f.ownerRole}
	_, err := f.svc.SetLogicalReplication(ctx, enable)
	if codeOf(err) != LogicalReplicationSubscriptionsExistCode || !strings.Contains(err.Error(), f.dbname+"."+carried) {
		t.Fatalf("enable on a copy holding its source's subscription = %v, want %s naming it", err, LogicalReplicationSubscriptionsExistCode)
	}
	if f.spec() != nil || len(f.sink.events) != 0 {
		t.Fatalf("the refusal wrote: spec %+v, %d audit rows", f.spec(), len(f.sink.events))
	}
	execAs(t, f.superuser, fmt.Sprintf(`ALTER SUBSCRIPTION %s SET (slot_name = NONE)`, carried), fmt.Sprintf(`DROP SUBSCRIPTION %s`, carried))
	if _, err := f.svc.SetLogicalReplication(ctx, enable); err != nil {
		t.Fatalf("enable once the carried subscription is dropped = %v", err)
	}
	if lr := f.spec(); lr == nil || lr.SubscriberRole != f.ownerRole {
		t.Fatalf("spec = %+v, want the owner as the subscriber role", lr)
	}
}

// An HA publisher's enable answers the failover warning for its major, as
// read from the server itself (t002).
func TestHAPublisherEnableWarnsAboutFailover(t *testing.T) {
	f := newLRFixture(t)
	shortenLogicalReplicationWait(t, 300*time.Millisecond)
	ctx := context.Background()
	var d appv1alpha1.Database
	if err := f.svc.Client.Get(ctx, client.ObjectKeyFromObject(f.db), &d); err != nil {
		t.Fatal(err)
	}
	d.Spec.HighAvailability = true
	if err := f.svc.Client.Update(ctx, &d); err != nil {
		t.Fatal(err)
	}
	var major int
	if conn, err := pgx.Connect(ctx, f.superuser); err == nil {
		_ = conn.QueryRow(ctx, `SELECT current_setting('server_version_num')::int / 10000`).Scan(&major)
		_ = conn.Close(ctx)
	}
	res, err := f.svc.SetLogicalReplication(ctx, LogicalReplicationRequest{Database: f.db.Name, Enable: true, PublisherRole: f.readerRole, Schemas: []string{"public"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := failoverWarnings(&d, &appv1alpha1.DatabaseLogicalReplication{PublisherRole: f.readerRole}, major); len(want) != 1 || strings.Join(res.Warnings, "|") != want[0] {
		t.Fatalf("warnings = %q, want %q", res.Warnings, want)
	}
}

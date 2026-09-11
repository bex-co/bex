"""Execute Product adoption SQL and lifecycle triggers on disposable PostgreSQL."""
import json
import os
from pathlib import Path
import re
import subprocess
import unittest
import uuid

ROOT = Path(__file__).resolve().parents[1]


# Real schema dependencies, not copies of the analytics schema under test.
MIGRATIONS = ["0001_core", "0005_deploys", "0035_domain_redirect_for_name",
              "0086_domain_claim_state", "0095_apps_service_type",
              "0114_product_analytics", "0115_product_inventory",
              "0117_product_activity_surface"]


# Each view's column list as previously released, in order. CREATE OR REPLACE
# VIEW can only append, so every later generation must keep these exact prefixes
# or the bootstrap aborts against a cluster that already has the older view --
# which is to say, against production (w5/m94). Never reorder or edit these
# lists; only append new columns after them.
RELEASED_VIEW_COLUMNS = {
    "events": ["source_key", "workspace_id", "resource_id", "parent_id", "resource_type",
               "event_type", "at", "recorded_at", "actor_id", "actor_type", "provenance",
               "outcome", "duration_ms", "audience"],
    "collection": ["started_at", "inventory_started_at", "events_retained_from"],
}


class ProductAnalyticsTest(unittest.TestCase):
    @classmethod
    def psql(cls, sql, database=None, check=True):
        sql = re.sub(r"\bbex_product_analytics\b", cls.role, sql)
        result = subprocess.run(cls.prefix + ["-X", "-qAt", "-v", "ON_ERROR_STOP=1",
                                "-d", database or cls.database],
                                input=sql, text=True, capture_output=True)
        if check and result.returncode:
            raise AssertionError(result.stderr)
        return result

    @classmethod
    def setUpClass(cls):
        container = os.environ.get("BEX_CLI_ANALYTICS_TEST_CONTAINER")
        cls.prefix = (["docker", "exec", "-i", container, "psql", "-U", "postgres"]
                      if container else ["psql"])
        cls.database = "bex_product_test_" + uuid.uuid4().hex
        cls.role = "bex_product_reader_" + uuid.uuid4().hex
        cls.psql(f'CREATE DATABASE "{cls.database}"', "postgres")
        cls.addClassCleanup(cls.cleanup_database)
        migrations = ROOT / "lego/backend/internal/store/migrations"
        # Real schema dependencies, not copies of the analytics schema under test.
        for name in MIGRATIONS:
            cls.psql((migrations / (name + ".up.sql")).read_text())
        cls.psql((ROOT / "scripts/product-analytics.sql").read_text())
        cls.psql((ROOT / "scripts/product-analytics.sql").read_text())
        values = (ROOT / "deploy/gitops/base/values/grafana.values.yaml").read_text()
        match = re.search(r"^    product-adoption:\n      json: \|\n((?:        .*\n|\n)+)", values, re.M)
        cls.board = json.loads(match.group(1))
        cls.panels = {p["id"]: p for p in cls.board["panels"]}
        for panel in cls.board["panels"]:
            cls.panels.update({p["id"]: p for p in panel.get("panels", [])})
        cls.psql("""
          UPDATE product_analytics_collection SET started_at='2026-08-01',inventory_started_at='2026-09-01',surface_started_at='2026-09-01';
          INSERT INTO tenants(id,name,created_at) VALUES
            ('w1','one','2026-09-01'),('w2','two','2026-09-02'),('internal','internal','2026-09-01');
          INSERT INTO product_analytics_audiences VALUES ('w1','customer'),('internal','qa');
          INSERT INTO apps(id,tenant_id,name,repo,type,created_at) VALUES
            ('a1','w1','static','repo','static_site','2026-09-03'),
            ('a2','w1','web','repo','web_service','2026-09-04'),
            ('a3','w2','web','repo','web_service','2026-09-04'),
            ('qa-app','internal','qa','repo','static_site','2026-09-04');
          INSERT INTO product_activity_events(source_key,workspace_id,resource_id,resource_type,event_type,at,actor_id,actor_type,surface) VALUES
            ('created:a1','w1','a1','static_site','created','2026-09-03','user-one','human','dashboard'),
            ('created:a2','w1','a2','web_service','created','2026-09-04','user-one','human','mcp'),
            ('created:a3','w2','a3','web_service','created','2026-09-04','api-key','machine','mcp'),
            ('created:qa-app','internal','qa-app','static_site','created','2026-09-04','qa-user','human','cli');
          INSERT INTO deploys(id,app_id,trigger,status,created_at,started_at,finished_at) VALUES
            ('d1','a1','create','live','2026-09-03','2026-09-04 12:00','2026-09-04 12:01'),
            ('d2','a2','create','build_failed','2026-09-04','2026-09-05 12:00','2026-09-05 12:01:30'),
            ('d3','a3','api','canceled','2026-09-06',NULL,'2026-09-06 12:00');
          UPDATE deploys SET status='deactivated' WHERE id='d1';
          INSERT INTO domains(id,app_id,host,redirect_for_name,created_at,claim_state,challenge) VALUES
            ('domain1','a1','example.test','','2026-09-04','pending','proof'),
            ('alias1','a1','www.example.test','example.test','2026-09-04','pending','proof');
          UPDATE domains SET claim_state='verified',verified_at='2026-09-05' WHERE id='domain1';
          UPDATE domains SET verification_attempts=verification_attempts+1 WHERE id='domain1';
          INSERT INTO product_hosting_daily VALUES
            ('w1','a1','static_site','2026-09-04','2026-09-04 12:01',true,true),
            ('w1','a2','web_service','2026-09-04','2026-09-04 12:01',false,false),
            ('w1','a1','static_site','2026-09-05','2026-09-05 12:01',false,true),
            ('internal','qa-app','static_site','2026-09-05','2026-09-05 12:01',true,true);
          INSERT INTO product_domain_observations VALUES ('domain1',now(),true,now());
          INSERT INTO product_inventory_batches(source,bucket,observed_at,complete)
          SELECT s,t,t,true FROM unnest(ARRAY['services','postgres','keyvalue','domains']) s,
          unnest(ARRAY['2026-09-03 11:55','2026-09-04 23:55','2026-09-05 23:55',
                      '2026-09-06 23:55','2026-09-08 23:55','2026-09-10 11:55']::timestamptz[]) t;
          UPDATE product_inventory_batches SET complete=false WHERE source='postgres' AND bucket='2026-09-06 23:55';
          INSERT INTO product_inventory_counts
          SELECT 'services',bucket,'w1','static_site','ready',CASE WHEN bucket>='2026-09-08' THEN 2 ELSE 1 END
          FROM product_inventory_batches WHERE source='services';
          INSERT INTO product_inventory_counts
          SELECT 'services',bucket,'w2','web_service','suspended',1 FROM product_inventory_batches WHERE source='services';
          INSERT INTO product_inventory_counts
          SELECT 'services',bucket,'w1','web_service','failed',1 FROM product_inventory_batches
          WHERE source='services' AND bucket<'2026-09-05';
          INSERT INTO product_inventory_counts
          SELECT 'services',bucket,'internal','static_site','ready',1 FROM product_inventory_batches WHERE source='services';
          INSERT INTO product_inventory_counts
          SELECT 'postgres',bucket,'w1','postgres',CASE WHEN bucket<'2026-09-08' THEN 'failed' ELSE 'ready' END,1
          FROM product_inventory_batches WHERE source='postgres' AND complete AND bucket>'2026-09-04';
          INSERT INTO product_inventory_counts
          SELECT 'keyvalue',bucket,'w2','keyvalue','provisioning',1 FROM product_inventory_batches
          WHERE source='keyvalue' AND bucket>'2026-09-08';
          INSERT INTO product_inventory_counts
          SELECT 'domains',bucket,'w1','static_site',CASE WHEN bucket='2026-09-05 23:55' THEN 'verified_tls_unknown' ELSE 'verified_tls_ready' END,1
          FROM product_inventory_batches WHERE source='domains' AND bucket>'2026-09-04';
          INSERT INTO product_inventory_lifecycle VALUES
          ('a1','w1','services','static_site','2026-09-03','2026-09-03 11:55','2026-09-10 11:55','2026-09-04 12:01',NULL,'ready'),
          ('a2','w1','services','web_service','2026-09-04','2026-09-04','2026-09-04 23:55',NULL,'2026-09-05 23:55','failed'),
          ('a3','w2','services','web_service','2026-09-04','2026-09-04','2026-09-10 11:55',NULL,NULL,'suspended'),
          ('pg1','w1','postgres','postgres','2026-09-04','2026-09-04','2026-09-10 11:55','2026-09-08 10:00',NULL,'ready'),
          ('kv1','w2','keyvalue','keyvalue','2026-09-08','2026-09-08','2026-09-10 11:55',NULL,NULL,'provisioning');
        """)

    @classmethod
    def cleanup_database(cls):
        cls.psql(f'DROP DATABASE "{cls.database}" WITH (FORCE)', "postgres")
        cls.psql("DROP ROLE IF EXISTS bex_product_analytics", "postgres")

    def query(self, panel, start="2026-09-01", end="2026-09-10 12:00", kind="__all",
              audience="'customer','unclassified'", resolution="1 day"):
        sql = self.panels[panel]["targets"][0]["rawSql"]
        sql = re.sub(r"\$__timeFilter\((\w+)\)",
                     lambda m: f"{m[1]} >= '{start}'::timestamptz AND {m[1]} <= '{end}'::timestamptz", sql)
        sql = sql.replace("$__timeFrom()", f"'{start}'").replace("$__timeTo()", f"'{end}'")
        sql = sql.replace("${kind:sqlstring}", "'" + kind.replace("'", "''") + "'")
        sql = sql.replace("${audience:sqlstring}", audience)
        sql = sql.replace("${resolution:sqlstring}", "'" + resolution.replace("'", "''") + "'")
        result = self.psql("SET ROLE bex_product_analytics; SET timezone='UTC'; "
                          + "SELECT COALESCE(json_agg(q),'[]'::json) FROM (" + sql + ") q")
        return json.loads(result.stdout)

    def test_all_panel_queries(self):
        for panel in self.panels.values():
            if panel.get("targets"):
                with self.subTest(panel=panel["title"]):
                    self.query(panel["id"])

    def point(self, panel, time, metric, **kwargs):
        rows = [r for r in self.query(panel, **kwargs)
                if r["time"].startswith(time) and r["metric"] == metric]
        self.assertEqual(len(rows), 1, rows)
        return rows[0]["value"]

    def test_inventory_counts_existing_not_just_ready(self):
        self.assertEqual(self.point(104, "2026-09-05", "postgres"), 1)
        self.assertEqual(self.point(108, "2026-09-05", "failed"), 2)
        self.assertEqual(self.point(103, "2026-09-06", "web_service"), 1)
        self.assertEqual(self.point(108, "2026-09-06", "suspended"), 1)
        self.assertEqual(self.point(103, "2026-09-10T12:00", "static_site"), 2)
        self.assertEqual(self.point(104, "2026-09-05", "keyvalue"), 0)

    def test_failed_missing_and_stale_samples_are_gaps(self):
        self.assertIsNone(self.point(104, "2026-09-07", "postgres"))
        self.assertIsNone(self.point(105, "2026-09-07", "All selected features"))
        self.assertEqual(self.point(103, "2026-09-07", "static_site"), 1)
        self.assertIsNone(self.point(103, "2026-09-08", "static_site"))
        self.assertIsNone(self.point(103, "2026-09-10T13:00", "static_site", end="2026-09-10 13:00"))
        coverage = self.query(102, end="2026-09-10 13:00")
        self.assertTrue(all(r["Inventory coverage"] == "Stale" for r in coverage))

    def test_workspace_reach_is_distinct_and_filters_are_consistent(self):
        self.assertEqual(self.point(105, "2026-09-10T12:00", "All selected features"), 2)
        self.assertEqual(self.point(105, "2026-09-10T12:00", "static_site"), 1)
        self.assertEqual(self.point(105, "2026-09-10T12:00", "All selected features", audience="'qa'"), 1)
        self.assertEqual(self.point(105, "2026-09-07", "All selected features", kind="static_site"), 1)
        self.assertEqual(self.point(103, "2026-09-10T12:00", "static_site", audience="'qa'"), 1)

    def test_domain_subsets_and_unknown_tls(self):
        self.assertEqual(self.point(106, "2026-09-06", "Configured"), 1)
        self.assertEqual(self.point(106, "2026-09-06", "Ownership verified"), 1)
        self.assertEqual(self.point(106, "2026-09-06", "Certificate-ready (observed)"), 0)
        self.assertEqual(self.point(106, "2026-09-06", "Certificate unknown"), 1)
        self.assertEqual(self.point(106, "2026-09-06", "Configured", kind="web_service"), 0)

    def test_net_change_requires_both_complete_endpoints(self):
        rows = {r["Type"]: r for r in self.query(111)}
        self.assertEqual(rows["static_site"]["At range end"], 2)
        self.assertEqual(rows["static_site"]["Owning workspaces"], 1)
        self.assertEqual(rows["static_site"]["7-day net change"], 1)
        self.assertEqual(rows["web_service"]["7-day net change"], -1)
        rows = self.query(111, end="2026-09-09")
        self.assertTrue(all(r["7-day net change"] is None for r in rows))

    def test_provisioning_does_not_hide_unfinished_or_peek_at_future_success(self):
        rows = {r["Type"]: r for r in self.query(109)}
        self.assertEqual(rows["static_site"]["Reached ready"], 1)
        self.assertEqual(rows["web_service"]["Removed before ready"], 1)
        self.assertEqual(rows["web_service"]["Unresolved"], 1)
        past = {r["Type"]: r for r in self.query(109, end="2026-09-06")}
        self.assertEqual(past["postgres"]["Reached ready"], 0)
        self.assertEqual(past["postgres"]["Unresolved"], 1)
        timing = {r["Type"]: r for r in self.query(110)}
        self.assertEqual(timing["static_site"]["Timed samples"], 1)
        self.assertEqual(timing["static_site"]["p50"], 129660)
        self.assertIsNone(timing["static_site"]["p95 (20+ samples)"])
        self.assertEqual(timing["web_service"]["Oldest unresolved"], 561600)

    def test_last_snapshot_not_daily_peak_or_sum(self):
        # A higher earlier snapshot must not override the last complete one.
        self.psql("""INSERT INTO product_inventory_batches VALUES ('services','2026-09-05 10:00','2026-09-05 10:00',true);
          INSERT INTO product_inventory_counts VALUES ('services','2026-09-05 10:00','w1','static_site','ready',99);""")
        try:
            self.assertEqual(self.point(103, "2026-09-06", "static_site"), 1)
        finally:
            self.psql("DELETE FROM product_inventory_batches WHERE source='services' AND bucket='2026-09-05 10:00'")

    def test_partial_day_changes_do_not_double_count(self):
        self.psql("""INSERT INTO product_activity_events(source_key,workspace_id,resource_id,resource_type,event_type,at)
          VALUES ('created:boundary','w1','boundary','static_site','created','2026-09-09 18:00');""")
        try:
            self.assertEqual(self.point(107, "2026-09-10T00:00", "Created"), 1)
            self.assertEqual(self.point(107, "2026-09-10T12:00", "Created"), 0)
        finally:
            self.psql("DELETE FROM product_activity_events WHERE source_key='created:boundary'")

    def test_expired_or_partially_collected_change_buckets_are_unknown(self):
        self.psql("UPDATE product_analytics_collection SET events_retained_from='2026-09-05 12:00'")
        try:
            self.assertIsNone(self.point(107,"2026-09-05","Created"))
            self.assertIsNone(self.point(107,"2026-09-06","Created"))
            self.assertEqual(self.point(107,"2026-09-07","Created"),0)
        finally:
            self.psql("UPDATE product_analytics_collection SET events_retained_from=NULL")

    def test_inventory_resolution_and_quoted_filters(self):
        for panel in range(103, 109):
            self.query(panel, start="2026-09-10 11:50", resolution="5 minutes")
            self.query(panel, start="2026-09-10 11:00", resolution="1 hour")
            self.query(panel, kind="web_service' OR true --", audience="'not-an-audience'")
        self.assertTrue(self.board["panels"][-1]["collapsed"])
        for panel in self.board["panels"]:
            if panel["type"] == "timeseries":
                self.assertFalse(panel["fieldConfig"]["defaults"]["custom"]["spanNulls"])
                self.assertEqual(panel["options"]["legend"]["calcs"], ["last"])

    def test_creators_not_resources_or_machine_accounts(self):
        self.assertEqual(self.query(2), [{"Creator accounts": 1}])
        self.assertEqual(self.query(3), [{"Creating workspaces": 2}])
        features = {r["Feature"]: r for r in self.query(6)}
        self.assertEqual(features["web_service"]["Created"], 2)
        self.assertEqual(features["web_service"]["Creators"], 1)
        self.assertEqual(features["web_service"]["Workspaces"], 2)
        self.assertEqual(features["static_site"]["Created"], 1)
        self.assertEqual(self.query(2, kind="static_site"), [{"Creator accounts": 1}])
        self.assertEqual(self.query(2, audience="'qa'"), [{"Creator accounts": 1}])

    def test_long_ranges_coarsen_fine_resolution_without_fabricating_history(self):
        for start in ["2026-08-01", "2026-01-01", "2020-01-01"]:
            for panel in range(103, 109):
                with self.subTest(start=start, panel=panel):
                    rows = self.query(panel, start=start, resolution="5 minutes")
                    points = {row["time"] for row in rows}
                    self.assertGreater(len(points), 1)
                    self.assertLessEqual(len(points), 1001)
                    self.assertTrue(all(row["value"] is None for row in rows
                                        if row["time"] < "2026-08-01"))

    def test_activated_and_hosting_are_distinct_workspaces(self):
        self.assertEqual(self.query(4), [{"Newly activated workspaces": 1}])
        self.assertEqual(self.query(5), [{"Hosting workspaces": 1}])
        daily = self.query(8)
        self.assertEqual(daily[3]["Hosting"], 1)
        self.assertEqual(daily[4]["Hosting"], 1)

    def test_terminal_replay_and_deactivation_do_not_double_count(self):
        rows = {r["Type"]: r for r in self.query(11)}
        self.assertEqual(rows["static_site"]["Finished"], 1)
        self.assertEqual(rows["static_site"]["p50"], 60000)
        self.assertEqual(rows["web_service"]["Finished"], 2)
        self.assertEqual(rows["web_service"]["Failed"], 1)
        self.assertEqual(rows["web_service"]["Failure share"], 1)
        self.assertEqual(rows["web_service"]["Timed samples"], 1)

    def test_domains_exclude_generated_aliases_and_repeat_verification(self):
        row = self.query(12)[0]
        self.assertEqual(row["Explicit domains"], 1)
        self.assertEqual(row["Ownership verified"], 1)
        self.assertEqual(row["TLS observed ready"], 1)
        historical = self.query(13)[0]
        self.assertEqual(historical["Domains added"], 1)
        self.assertEqual(historical["Verified by range end"], 1)
        self.assertEqual(historical["p50 verification time"], 86400)
        self.assertEqual(historical["Creator accounts"], 0)

    def test_cohorts_require_full_followup_window(self):
        self.assertEqual(self.query(9)[0]["Mature workspaces"], 2)
        self.assertEqual(self.query(9)[0]["Activation rate"], 0.5)
        self.assertEqual(self.query(9, end="2026-09-03")[0]["Mature workspaces"], 0)
        self.assertEqual(self.query(9, end="2026-09-03")[0]["Activation rate"], None)
        self.assertEqual(self.query(14), [])

    def test_empty_and_quoted_filters(self):
        for panel in [2, 3, 4, 5, 7, 9, 10, 11, 14, 15]:
            self.query(panel, kind="web_service' OR true --")
        self.assertEqual(self.query(2, start="2027-01-01", end="2027-01-02"),
                         [{"Creator accounts": 0}])

    def test_resource_delete_preserves_facts_but_failed_create_does_not(self):
        result = self.psql("""
          BEGIN;
          DELETE FROM apps WHERE id='a1';
          SELECT count(*) FROM product_activity_events WHERE source_key='created:a1';
          SELECT count(*) FROM product_activity_events WHERE source_key='deploy_finished:d1';
          SELECT count(*) FROM product_activity_events WHERE source_key='deleted:a1';
          INSERT INTO apps(id,tenant_id,name,repo,type) VALUES ('rollback','w1','rollback','repo','web_service');
          INSERT INTO deploys(id,app_id,trigger,status) VALUES ('rollback-deploy','rollback','create','created');
          DELETE FROM apps WHERE id='rollback';
          DELETE FROM product_activity_events WHERE resource_id='rollback' OR parent_id='rollback';
          SELECT count(*) FROM product_activity_events WHERE parent_id='rollback';
          ROLLBACK;
        """)
        self.assertEqual(result.stdout.splitlines(), ["1", "1", "1", "0"])

    def test_workspace_delete_removes_retained_identity_scope(self):
        result = self.psql("""
          BEGIN; DELETE FROM tenants WHERE id='w1';
          SELECT count(*) FROM product_activity_events WHERE workspace_id='w1';
          SELECT count(*) FROM product_hosting_daily WHERE workspace_id='w1';
          ROLLBACK;
        """)
        self.assertEqual(result.stdout.splitlines(), ["0", "0"])

    def test_expired_creation_is_not_failed_provisioning(self):
        result = self.psql("""
          BEGIN;
          DELETE FROM product_activity_events WHERE source_key='created:a1';
          DELETE FROM apps WHERE id='a1';
          SELECT count(*) FROM product_activity_events WHERE source_key='deploy_finished:d1';
          SELECT count(*) FROM product_activity_events WHERE source_key='deleted:a1';
          ROLLBACK;
        """)
        self.assertEqual(result.stdout.splitlines(), ["1", "1"])

    def test_explicit_alias_promotion_uses_adoption_time(self):
        result = self.psql("""
          BEGIN;
          UPDATE domains SET redirect_for_name='' WHERE id='alias1';
          SELECT at >= now() FROM product_activity_events WHERE source_key='domain_added:alias1';
          ROLLBACK;
        """)
        self.assertEqual(result.stdout.strip(), "t")

    def test_migration_down_up_and_legacy_backfill(self):
        migrations = ROOT / "lego/backend/internal/store/migrations"
        result = self.psql("BEGIN; DROP SCHEMA product_analytics CASCADE; "
            + (migrations / "0115_product_inventory.down.sql").read_text()
            + (migrations / "0114_product_analytics.down.sql").read_text()
            + (migrations / "0114_product_analytics.up.sql").read_text()
            + (migrations / "0115_product_inventory.up.sql").read_text()
            + "SELECT count(*) FROM product_activity_events WHERE event_type='created' AND resource_type='static_site' AND provenance='legacy' AND actor_type='unknown'; ROLLBACK;")
        self.assertEqual(result.stdout.strip(), "2")

    def test_reader_denies_raw_data_and_writes(self):
        for statement in ["SELECT * FROM apps", "SELECT * FROM product_activity_events",
                          "SELECT * FROM domains", "DELETE FROM product_analytics.events",
                          "SELECT * FROM product_inventory_counts", "DELETE FROM product_analytics.inventory",
                          "CREATE TABLE product_analytics.bad (x int)"]:
            with self.subTest(statement=statement):
                self.assertNotEqual(self.psql("SET ROLE bex_product_analytics; " + statement, check=False).returncode, 0)

    def test_view_columns_are_append_only(self):
        # CREATE OR REPLACE VIEW can only APPEND. w5/m94 shipped a projection
        # inserted mid-list, which aborts the bootstrap transaction on any
        # cluster already holding the previous generation -- production.
        #
        # Pinning the released prefix is what catches that. Deriving a "previous"
        # generation from the current file cannot: the shrunk copy inherits the
        # same bad ordering and re-applies cleanly (verified -- that approach was
        # tried here and silently passed the mutation).
        for view, released in RELEASED_VIEW_COLUMNS.items():
            with self.subTest(view=view):
                columns = self.psql(
                    "SELECT string_agg(column_name,' ' ORDER BY ordinal_position) "
                    "FROM information_schema.columns WHERE table_schema='product_analytics' "
                    f"AND table_name='{view}'").stdout.strip().split()
                self.assertEqual(columns[:len(released)], released,
                                 f"{view}: released columns must keep their order; only append after them")

    def test_reader_columns_are_an_explicit_allowlist(self):
        expected = {
            "events": "source_key workspace_id resource_id parent_id resource_type event_type at recorded_at actor_id actor_type provenance outcome duration_ms audience surface",
            "workspaces": "workspace_id created_at audience",
            "hosting": "workspace_id resource_id resource_type day observed_at live was_live audience",
            "domains": "domain_id resource_id workspace_id resource_type created_at claim_state verified_at verification_attempts observed_at tls_ready first_tls_ready_at audience",
            "collection": "started_at inventory_started_at events_retained_from surface_started_at",
            "inventory_batches": "source bucket observed_at complete",
            "inventory": "source bucket workspace_id resource_type state resources audience",
            "provisioning": "resource_id workspace_id source resource_type created_at first_seen_at last_seen_at first_ready_at removed_at state audience",
        }
        tables = ["product_activity_events", "product_hosting_daily",
                  "product_inventory_batches", "product_inventory_counts",
                  "product_inventory_lifecycle"]
        try:
            for table in tables:
                self.psql(f"ALTER TABLE {table} ADD COLUMN private_future_field text")
            self.psql((ROOT / "scripts/product-analytics.sql").read_text())
            result = self.psql("""SELECT json_object_agg(table_name, columns) FROM (
              SELECT table_name, string_agg(column_name,' ' ORDER BY ordinal_position) columns
              FROM information_schema.columns WHERE table_schema='product_analytics'
              GROUP BY table_name) v""")
            self.assertEqual(json.loads(result.stdout), expected)
        finally:
            for table in tables:
                self.psql(f"ALTER TABLE {table} DROP COLUMN IF EXISTS private_future_field")

    def test_creations_by_surface_are_exclusive_and_audience_filtered(self):
        # Fixtures: a1 dashboard (w1), a2 mcp (w1), a3 mcp (w2), qa-app cli (qa).
        # Default audience excludes qa, so the CLI row must not appear.
        rows = self.query(16)
        by_surface = {}
        for r in rows:
            by_surface[r["metric"]] = by_surface.get(r["metric"], 0) + r["value"]
        self.assertEqual(by_surface, {"dashboard": 1, "mcp": 2})
        # Exclusive categories, so unlike the overlapping feature lines these sum
        # to the total creation count.
        self.assertEqual(sum(by_surface.values()), 3)
        # The excluded audience's creation is reachable when it is selected.
        qa = {r["metric"] for r in self.query(16, audience="'qa'")}
        self.assertEqual(qa, {"cli"})

    def test_agent_driven_share_is_null_not_zero_without_creations(self):
        # Two of three in-audience creations arrived over MCP.
        self.assertAlmostEqual(self.query(17)[0]["Agent-driven share"], 2 / 3)
        # An empty window is an absence of evidence, not evidence of no agent
        # usage: the stat must read null rather than a confident 0%.
        empty = self.query(17, start="2026-01-01T00:00:00Z", end="2026-01-02T00:00:00Z")
        self.assertIsNone(empty[0]["Agent-driven share"])

    def test_agent_share_ignores_creations_predating_surface_collection(self):
        # Rows written before attribution began all default to 'unknown'. Left in
        # the denominator they would quietly deflate the share, which on a board
        # whose discipline is "gaps are labelled, never synthetic zeros" is the
        # worst failure mode: a confident, wrong, low number.
        self.psql("""
          INSERT INTO product_activity_events(source_key,workspace_id,resource_id,resource_type,event_type,at,actor_id,actor_type,provenance)
          VALUES ('created:legacy1','w1','legacy1','web_service','created','2026-08-15','','unknown','legacy'),
                 ('created:legacy2','w1','legacy2','web_service','created','2026-08-16','','unknown','legacy');
        """)
        self.addCleanup(lambda: self.psql(
            "DELETE FROM product_activity_events WHERE resource_id IN ('legacy1','legacy2')"))
        # Widened to cover the legacy rows; the score must not move.
        self.assertAlmostEqual(
            self.query(17, start="2026-08-01T00:00:00Z")[0]["Agent-driven share"], 2 / 3)
        # And a range entirely before collection began is unscorable, not 0%.
        before = self.query(17, start="2026-08-01T00:00:00Z", end="2026-08-20T00:00:00Z")
        self.assertIsNone(before[0]["Agent-driven share"])

    def test_dashboard_geometry_and_provisioning(self):
        for i, left in enumerate(self.board["panels"]):
            a = left["gridPos"]
            self.assertLessEqual(a["x"]+a["w"], 24)
            for right in self.board["panels"][i+1:]:
                b = right["gridPos"]
                self.assertFalse(a["x"] < b["x"]+b["w"] and b["x"] < a["x"]+a["w"]
                                 and a["y"] < b["y"]+b["h"] and b["y"] < a["y"]+a["h"])
        self.assertFalse(self.board["editable"])
        self.assertEqual(self.board["uid"], "bex-product-adoption")


    def test_collapsed_row_panels_do_not_overlap(self):
        # The geometry check above iterates board["panels"] only, so every child
        # of a collapsed row -- more than half this board -- was never checked.
        # Grafana reflows overlaps on load, which is exactly why they survive
        # review unnoticed.
        for row in self.board["panels"]:
            children = row.get("panels") or []
            for panel in children:
                rect = panel["gridPos"]
                self.assertLessEqual(rect["x"] + rect["w"], 24, panel["title"])
                for other in children:
                    if other["id"] <= panel["id"]:
                        continue
                    b = other["gridPos"]
                    overlap = (rect["x"] < b["x"] + b["w"] and b["x"] < rect["x"] + rect["w"]
                               and rect["y"] < b["y"] + b["h"] and b["y"] < rect["y"] + rect["h"])
                    self.assertFalse(overlap, (panel["title"], other["title"]))


if __name__ == "__main__":
    unittest.main()

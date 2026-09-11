"""Exercise the committed Grafana SQL on a disposable PostgreSQL database.

Local: set PGHOST/PGPORT/PGUSER for a disposable Postgres server.
CI: BEX_CLI_ANALYTICS_TEST_CONTAINER selects the ephemeral postgres service.
The suite creates and drops a uniquely named test database and reader role.
"""

import json
import os
from pathlib import Path
import re
import subprocess
import unittest
import uuid

ROOT = Path(__file__).resolve().parents[1]


# The view's column list exactly as w5/m92 released it. CREATE OR REPLACE VIEW
# can only APPEND columns, so a later generation that inserts, reorders or
# renames anything in this prefix fails against every cluster that already holds
# the older view -- production included, where the bootstrap script would abort
# mid-transaction. Never edit this list; only append below it.
RELEASED_VIEW_COLUMNS = [
    "received_at", "installation_id", "subject", "workspace_id", "command",
    "outcome", "failed", "duration_ms", "duration_mode", "upstream_version",
    "os", "arch", "output_format", "launched_full_screen_tui",
    "agent_signals", "ci_signals",
]


class CLIAnalyticsTest(unittest.TestCase):
    @classmethod
    def psql(cls, sql, database=None, check=True):
        # PostgreSQL roles are cluster-wide, even when databases are isolated.
        # Substitute only the reader identifier so concurrent suites cannot
        # alter or drop an existing production/development reader.
        sql = re.sub(r"\bbex_cli_analytics\b", cls.role, sql)
        result = subprocess.run(
            cls.prefix + ["-X", "-qAt", "-v", "ON_ERROR_STOP=1", "-d", database or cls.database],
            input=sql, text=True, capture_output=True,
        )
        if check and result.returncode:
            raise AssertionError(result.stderr)
        return result

    @classmethod
    def setUpClass(cls):
        container = os.environ.get("BEX_CLI_ANALYTICS_TEST_CONTAINER")
        cls.prefix = (["docker", "exec", "-i", container, "psql", "-U", "postgres"]
                      if container else ["psql"])
        cls.database = "bex_cli_analytics_test_" + uuid.uuid4().hex
        cls.role = "bex_cli_analytics_" + uuid.uuid4().hex
        cls.psql(f'CREATE DATABASE "{cls.database}"', "postgres")
        cls.addClassCleanup(cls.cleanup_database)
        cls.psql((ROOT / "lego/backend/internal/store/migrations/0113_cli_telemetry_events.up.sql").read_text())
        cls.psql((ROOT / "lego/backend/internal/store/migrations/0116_cli_telemetry_bex_version.up.sql").read_text())
        cls.psql("CREATE TABLE private_accounts (secret text); INSERT INTO private_accounts VALUES ('private')")
        cls.psql((ROOT / "scripts/cli-analytics.sql").read_text())
        # Applying twice must preserve access without duplicate objects.
        cls.psql((ROOT / "scripts/cli-analytics.sql").read_text())
        values = (ROOT / "deploy/gitops/base/values/grafana.values.yaml").read_text()
        match = re.search(r"^    cli-usage:\n      json: \|\n((?:        .*\n|\n)+)", values, re.M)
        cls.board = json.loads(match.group(1))
        cls.panels = {p["id"]: p for p in cls.board["panels"]}
        # ID, installation, UTC time, outcome, duration, exit, command, TUI, agents, CI.
        rows = [
            ("old-a", "a", "08-17 12:00", "success", 100, 0, "bex workspaces", False, "", ""),
            ("return-a", "a", "08-24 12:00", "success", 100, 0, "bex workspaces", False, "", ""),
            ("old-b", "b", "08-18 12:00", "success", 100, 0, "bex workspaces", False, "", ""),
            ("one", "a", "09-03 12:00", "success", 100, 0, "bex workspaces", False, "", ""),
            ("two", "a", "09-04 12:00", "execution_error", 300, 1, "bex workspaces", False, "CLAUDECODE,CURSOR_AGENT,CURSOR_TRACE_ID", "CI,GITHUB_ACTIONS"),
            ("help", "a", "09-04 13:00", "help", 100000, 0, "bex workspaces", False, "", ""),
            ("three", "b", "09-05 12:00", "success", 200, 0, "bex workspaces", False, "", ""),
            ("four", "c", "09-06 12:00", "explicit_exit", 500, 2, "bex workspaces", False, "CODEX_THREAD_ID", "CI"),
            ("empty", "  ", "09-07 12:00", "validation_error", 0, 1, "bex workspaces", False, "", ""),
            ("stream", "d", "09-08 12:00", "success", 3600000, 0, "bex logs", False, "", ""),
            ("interactive", "e", "09-09 12:00", "success", 60000, 0, "bex workspaces", True, "", ""),
            ("negative", "f", "09-10 10:00", "execution_error", -20, 1, "bex workspaces", False, "", ""),
            ("unknown", "g", "09-10 11:00", "future_kind", 200, 1, "bex workspaces", False, "", ""),
            ("future", "a", "09-11 12:00", "success", 100, 0, "bex workspaces", False, "", ""),
        ]
        # The bex launcher release is a second version axis beside the upstream
        # pin every fixture shares (w5/m94). Most rows come from one release, a
        # couple from the next, and two send nothing at all -- an unmodified
        # upstream `render` binary and a pre-m94 bex build, which the view must
        # report as 'unknown' rather than hide.
        bex_releases = {"one": "0.2.2", "two": "0.2.2", "three": "", "stream": ""}
        for event_id, install, when, outcome, duration, exit_code, command, tui, agents, ci in rows:
            bex_version = bex_releases.get(event_id, "0.2.1")
            # These literals are fixed fixtures, not user input. Shared subject and
            # workspace deliberately prove that installations are not people.
            cls.psql(f"""INSERT INTO cli_telemetry_events
                (id, installation_id, subject, workspace_id, received_at,
                 command, completion_kind, duration_ms, exit_code,
                 launched_full_screen_tui, agent_signals, ci_signals,
                 cli_version, bex_version, os, arch, output_format)
                VALUES ('{event_id}', '{install}', 'one-identity', 'tea-one',
                 '2026-{when}+00', '{command}', '{outcome}', {duration}, {exit_code},
                 {str(tui).lower()}, '{agents}', '{ci}', '2.27.0', '{bex_version}',
                 'linux', 'arm64', 'json')""")

    @classmethod
    def cleanup_database(cls):
        cls.psql(f'DROP DATABASE "{cls.database}" WITH (FORCE)', "postgres")
        cls.psql("DROP ROLE IF EXISTS bex_cli_analytics", "postgres")

    def query(self, panel_id, start="2026-09-03T00:00:00Z", end="2026-09-10T12:00:00Z",
              command="__all", bexrelease="__all"):
        sql = self.panels[panel_id]["targets"][0]["rawSql"]
        sql = sql.replace("$__timeFilter(received_at)", f"received_at >= '{start}' AND received_at <= '{end}'")
        sql = sql.replace("$__timeFrom()", f"'{start}'").replace("$__timeTo()", f"'{end}'")
        for variable, value in (("command", command), ("version", "__all"),
                                ("bexrelease", bexrelease), ("output", "__all")):
            sql = sql.replace("${" + variable + ":sqlstring}", "'" + value.replace("'", "''") + "'")
        return self.read_as_reader(sql)

    def read_as_reader(self, sql):
        """Run sql as the restricted reader, in UTC, decoded."""
        result = self.psql("SET ROLE bex_cli_analytics; SET timezone = 'UTC'; "
                           + "SELECT coalesce(json_agg(q), '[]'::json) FROM (" + sql + ") q")
        return json.loads(result.stdout)

    def test_every_committed_query_executes(self):
        for panel_id, panel in self.panels.items():
            if panel.get("targets", [{}])[0].get("rawSql"):
                with self.subTest(panel=panel["title"]):
                    self.query(panel_id)
        for variable in self.board["templating"]["list"]:
            self.psql("SET ROLE bex_cli_analytics; " + variable["query"])

    def test_counts_and_failure_denominator(self):
        self.assertEqual(self.query(2), [{"Observed commands": 10}])
        self.assertEqual(self.query(3), [{"Active installations": 7}])
        self.assertEqual(self.query(4), [{"Active workspaces": 1}])
        self.assertEqual(self.query(5), [{"Command failure share": 0.5}])
        daily = self.query(6)
        self.assertEqual(daily[1]["installations"], 1)
        self.assertEqual(daily[1]["identities"], 1)
        self.assertEqual(daily[4]["installations"], 0)

    def test_command_duration_excludes_help_invalid_and_sessions(self):
        command = self.query(8)[0]
        self.assertEqual(command["Command"], "bex workspaces")
        self.assertEqual(command["Commands"], 9)
        self.assertEqual(command["Installations"], 6)
        self.assertEqual(command["Affected installations"], 3)
        self.assertEqual(command["Duration samples"], 4)
        self.assertEqual(command["p50"], 250)
        self.assertAlmostEqual(command["p95"], 470)
        modes = {row["Mode"]: row for row in self.query(16)}
        self.assertEqual(modes["session / stream"]["p95"], 3600000)
        self.assertEqual(modes["interactive"]["p95"], 60000)

    def test_agent_overlap_and_ci_deduplication(self):
        agents = {row["Agent"]: row["Commands"] for row in self.query(10)}
        self.assertEqual(agents, {"Claude Code": 1, "Cursor": 1, "Codex": 1})
        ci = {row["CI"]: row["Commands"] for row in self.query(11)}
        self.assertEqual(ci, {"GitHub Actions": 1, "Generic / other": 1})
        day = self.query(9)[1]
        self.assertEqual(day["Agent detected"], 0.5)
        self.assertEqual(day["CI detected"], 0.5)

    def test_retention_deduplicates_and_excludes_immature_cohorts(self):
        self.assertEqual(self.query(13), [])
        cohorts = self.query(13, start="2026-08-01T00:00:00Z")
        self.assertEqual(len(cohorts), 1)
        self.assertEqual(cohorts[0]["Installations"], 2)
        self.assertEqual(cohorts[0]["Returned next week"], 1)
        self.assertEqual(cohorts[0]["Week 1 return"], 0.5)
        daily = self.query(12)
        self.assertEqual(daily[1]["First seen"], 0)
        self.assertEqual(daily[1]["Returning"], 1)
        self.assertEqual(daily[3]["First seen"], 1)

    def test_rolling_activity_includes_history_before_visible_range(self):
        day = self.query(14)[0]
        self.assertEqual(day["1 day"], 1)
        self.assertEqual(day["7 days"], 1)
        self.assertEqual(day["30 days"], 2)

    def test_empty_selection_has_zero_counts_but_no_failure_rate(self):
        self.assertEqual(self.query(2, command="missing"), [{"Observed commands": 0}])
        self.assertEqual(self.query(3, command="missing"), [{"Active installations": 0}])
        self.assertEqual(self.query(5, command="missing"), [{"Command failure share": None}])
        self.assertEqual(self.query(13, command="missing"), [])
        self.assertEqual(self.query(9, command="missing")[0]["Agent detected"], None)

    def test_filters_and_quoted_command(self):
        self.assertEqual(self.query(2, command="bex logs"), [{"Observed commands": 1}])
        self.assertEqual(self.query(5, command="bex logs"), [{"Command failure share": 0}])
        self.assertEqual(self.query(2, command="x'); DROP VIEW cli_analytics.events; --"), [{"Observed commands": 0}])
        self.assertEqual(self.query(2)[0]["Observed commands"], 10)

    def test_bex_release_is_an_axis_independent_of_the_upstream_pin(self):
        # Every fixture shares one upstream pin, so any split the reader sees
        # here can only come from the launcher's own release (w5/m94).
        rows = self.read_as_reader("SELECT bex_version, upstream_version, count(*) AS n "
                                   "FROM cli_analytics.events GROUP BY 1, 2 ORDER BY 1")
        self.assertEqual({r["upstream_version"] for r in rows}, {"2.27.0"})
        self.assertEqual({r["bex_version"]: r["n"] for r in rows},
                         {"0.2.1": 10, "0.2.2": 2, "unknown": 2})

    def test_release_adoption_panel_counts_installations_per_release(self):
        rows = self.query(25)
        by_release = {r["Bex release"]: r for r in rows}
        # Fixtures: ten rows on 0.2.1, two on 0.2.2, two with no header. Only
        # the rows inside the queried window are counted, so this also pins that
        # the panel respects the range rather than reporting all-time totals.
        self.assertEqual(set(by_release), {"0.2.1", "0.2.2", "unknown"})
        self.assertEqual(by_release["0.2.2"]["Installations"], 1)
        # Ten of the fourteen fixture rows fall inside the queried window; two
        # are 0.2.2 and two carry no header, leaving exactly six on 0.2.1. An
        # all-time total would read ten, so this pins range-scoping too.
        self.assertEqual(by_release["0.2.1"]["Commands"], 6)

    def test_bex_release_filter_scopes_product_panels(self):
        # Selecting one release must narrow the product panels; a release with
        # no rows in range must read as zero rather than falling back to all.
        everything = self.query(2)[0]["Observed commands"]
        one_release = self.query(2, bexrelease="0.2.2")[0]["Observed commands"]
        self.assertLess(one_release, everything)
        self.assertGreater(one_release, 0)
        self.assertEqual(self.query(2, bexrelease="not-a-release")[0]["Observed commands"], 0)

    def test_view_columns_are_append_only(self):
        rows = self.psql("SELECT column_name FROM information_schema.columns "
                         "WHERE table_schema = 'cli_analytics' AND table_name = 'events' "
                         "ORDER BY ordinal_position").stdout.split()
        self.assertEqual(rows[:len(RELEASED_VIEW_COLUMNS)], RELEASED_VIEW_COLUMNS)
        self.assertEqual(rows[len(RELEASED_VIEW_COLUMNS):], ["bex_version"])

    def test_every_product_panel_carries_the_release_filter(self):
        # The predicate was threaded into each panel by hand; without this the
        # next panel added would silently ignore the Bex release selector.
        for panel in self.panels.values():
            sql = (panel.get("targets") or [{}])[0].get("rawSql") or ""
            if "${command:sqlstring}" in sql:
                self.assertIn("${bexrelease:sqlstring}", sql, panel["title"])

    def test_collection_health_ignores_the_release_filter(self):
        # Last-event-received answers "is anything arriving at all", so a
        # product filter must not be able to make a healthy collector look dead.
        sql = self.panels[21]["targets"][0]["rawSql"]
        self.assertNotIn("bexrelease", sql)

    def test_reader_cannot_read_product_tables_or_mutate(self):
        for sql in (
            "SELECT * FROM private_accounts", "SELECT * FROM cli_telemetry_events",
            "DELETE FROM cli_analytics.events", "DROP VIEW cli_analytics.events",
            "CREATE TABLE cli_analytics.surprise (id int)",
        ):
            with self.subTest(sql=sql):
                result = self.psql("SET ROLE bex_cli_analytics; " + sql, check=False)
                self.assertNotEqual(result.returncode, 0)
                self.assertRegex(result.stderr, "permission denied|must be owner")
        result = self.psql("SELECT rolconnlimit, rolsuper, rolcreaterole, rolcreatedb, rolbypassrls, array_to_string(rolconfig, ',') FROM pg_roles WHERE rolname = 'bex_cli_analytics'").stdout
        self.assertIn("6|f|f|f|f|", result)
        self.assertIn("statement_timeout=10s", result)
        self.assertIn("default_transaction_read_only=on", result)

    def test_dashboard_geometry_and_datasource(self):
        self.assertEqual(len(self.panels), len(self.board["panels"]))
        self.assertEqual(self.board["time"]["from"], "now-7d")
        for panel in self.panels.values():
            rect = panel["gridPos"]
            self.assertLessEqual(rect["x"] + rect["w"], 24)
            for other in self.panels.values():
                if other["id"] <= panel["id"]:
                    continue
                b = other["gridPos"]
                overlap = (rect["x"] < b["x"] + b["w"] and b["x"] < rect["x"] + rect["w"]
                           and rect["y"] < b["y"] + b["h"] and b["y"] < rect["y"] + rect["h"])
                self.assertFalse(overlap, (panel["title"], other["title"]))


if __name__ == "__main__":
    unittest.main()

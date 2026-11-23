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


# Real schema dependencies, not copies: 0114 owns product_analytics_audiences,
# which the view joins for its audience column (ADR113 D9), and it needs the
# tables its triggers read. 0125 (the default-audience trigger) is left out so
# the fixtures below label workspaces by hand, as test_product_analytics does.
MIGRATIONS = ["0001_core", "0005_deploys", "0035_domain_redirect_for_name",
              "0086_domain_claim_state", "0095_apps_service_type",
              "0113_cli_telemetry_events", "0114_product_analytics",
              "0116_cli_telemetry_bex_version"]

# The instant the tests stand in for now(): the ops panels that read the
# present (retained window, clients by release) must not depend on the day the
# suite runs.
NOW = "2026-10-20T00:00:00Z"


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
        migrations = ROOT / "lego/backend/internal/store/migrations"
        for name in MIGRATIONS:
            cls.psql((migrations / (name + ".up.sql")).read_text())
        cls.psql("CREATE TABLE private_accounts (secret text); INSERT INTO private_accounts VALUES ('private')")
        # 'tea-one', the workspace of every September fixture, is no tenant and
        # so carries no label: those rows are 'unclassified' and stay in the
        # board's default audience.
        cls.psql("""INSERT INTO tenants (id, name) VALUES
            ('tea-cust', 'customer'), ('tea-qa', 'qa'), ('tea-int', 'internal');
            INSERT INTO product_analytics_audiences VALUES
            ('tea-cust', 'customer'), ('tea-qa', 'qa'), ('tea-int', 'internal')""")
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
        # October: the extended CLI, read by the ADR113 D7/D9 columns and
        # panels. Kept apart from the September window above so its counts stay
        # put. Week of 2026-10-05 is the mature activation cohort at the
        # 2026-10-20 end; 2026-10-12's is not yet.
        # ID, installation, workspace, UTC time, command, outcome, duration,
        # exit, agents, bex release, upstream, seconds from start to receipt.
        october = [
            ("o1", "n1", "tea-cust", "10-05 09:00", "bex services create", "success", 1000, 0, "", "0.2.2", "2.36.0", 3),
            ("o2", "n1", "tea-cust", "10-05 10:00", "bex postgres create", "execution_error", 500, 1, "", "0.2.2", "2.36.0", 1.5),
            ("o3", "n2", "tea-cust", "10-06 09:00", "bex agent run", "explicit_exit", 2000, 3, "CLAUDECODE", "0.2.2", "2.36.0", 4),
            ("o4", "n2", "tea-cust", "10-06 10:00", "bex agent wait", "explicit_exit", 3600000, 124, "CLAUDECODE", "0.2.2", "2.36.0", 3601),
            ("o5", "n2", "tea-cust", "10-06 11:00", "bex agent run", "help", 0, 0, "", "0.2.2", "2.36.0", None),
            ("o6", "n2", "tea-cust", "10-12 09:00", "bex agent run", "explicit_exit", 1500, 0, "CLAUDECODE", "0.2.2", "2.36.0", None),
            ("o7", "n3", "tea-cust", "10-07 09:00", "bex agent automations run", "explicit_exit", 100, 6, "", "0.2.2", "2.36.0", None),
            ("o8", "n3", "tea-cust", "10-07 10:00", "bex agent open", "success", 100, 0, "", "0.2.2", "2.36.0", None),
            ("o9", "n3", "tea-cust", "10-07 11:00", "bex glm", "success", 0, 0, "", "0.2.2", "2.36.0", None),
            # Eight days after n3 was first seen: too late to activate it.
            ("o10", "n3", "tea-cust", "10-15 09:00", "bex deploys create", "explicit_exit", 2000, 0, "", "0.2.2", "2.36.0", None),
            ("o11", "n4", "tea-qa", "10-06 09:00", "bex deploys create", "explicit_exit", 1000, 0, "", "0.2.2", "2.36.0", None),
            ("o12", "n4", "tea-qa", "10-06 10:00", "bex sandboxes exec", "explicit_exit", 100, 2, "", "0.2.2", "2.36.0", None),
            # No workspace attributed; the client clock runs five seconds ahead.
            ("o13", "n5", "", "10-08 09:00", "bex whoami", "success", 100, 0, "", "0.2.2", "2.36.0", -5),
            ("o14", "n6", "tea-cust", "10-08 09:00", "render services", "success", 300, 0, "", "", "2.30.0", None),
            ("o15", "n7", "tea-int", "10-09 09:00", "bex docs", "success", 100, 0, "", "0.2.2", "2.36.0", None),
            ("o16", "n8", "tea-cust", "10-13 09:00", "bex ea objects list", "success", 100, 0, "", "0.2.2", "2.36.0", None),
            ("o17", "n1", "tea-cust", "10-19 12:00", "bex services list", "success", 100, 0, "", "0.2.2", "2.36.0", None),
            ("o18", "n6", "tea-cust", "10-19 18:00", "render services", "success", 300, 0, "", "", "2.30.0", None),
        ]
        for (event_id, install, workspace, when, command, outcome, duration, exit_code,
             agents, bex_version, upstream, lag) in october:
            started = "NULL" if lag is None else f"'2026-{when}+00'::timestamptz - interval '{lag} seconds'"
            cls.psql(f"""INSERT INTO cli_telemetry_events
                (id, installation_id, subject, workspace_id, received_at, started_at,
                 command, completion_kind, duration_ms, exit_code, agent_signals,
                 cli_version, bex_version, os, arch, output_format)
                VALUES ('{event_id}', '{install}', 'one-identity', '{workspace}',
                 '2026-{when}+00', {started}, '{command}', '{outcome}', {duration},
                 {exit_code}, '{agents}', '{upstream}', '{bex_version}', 'linux',
                 'arm64', 'json')""")

    @classmethod
    def cleanup_database(cls):
        cls.psql(f'DROP DATABASE "{cls.database}" WITH (FORCE)', "postgres")
        cls.psql("DROP ROLE IF EXISTS bex_cli_analytics", "postgres")

    def query(self, panel_id, start="2026-09-03T00:00:00Z", end="2026-09-10T12:00:00Z",
              command="__all", bexrelease="__all", audience=("customer", "unclassified"),
              family="__all"):
        """Run a panel as Grafana would, with the board's default filters."""
        sql = self.panels[panel_id]["targets"][0]["rawSql"]
        sql = sql.replace("$__timeFilter(received_at)", f"received_at >= '{start}' AND received_at <= '{end}'")
        sql = sql.replace("$__timeFrom()", f"'{start}'").replace("$__timeTo()", f"'{end}'")
        sql = sql.replace("now()", f"'{NOW}'::timestamptz")
        for variable, value in (("command", command), ("version", "__all"),
                                ("bexrelease", bexrelease), ("output", "__all"),
                                ("audience", audience), ("family", family)):
            values = (value,) if isinstance(value, str) else value
            sql = sql.replace("${" + variable + ":sqlstring}",
                              ",".join("'" + v.replace("'", "''") + "'" for v in values))
        return self.read_as_reader(sql)

    def october(self, panel_id, **filters):
        """A panel over the October fixtures."""
        return self.query(panel_id, start="2026-10-05T00:00:00Z", end=NOW, **filters)

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
            if variable["type"] == "query":
                self.psql("SET ROLE bex_cli_analytics; " + variable["query"])
        for annotation in self.board["annotations"]["list"]:
            with self.subTest(annotation=annotation["name"]):
                self.read_as_reader(annotation["target"]["rawSql"])

    def test_release_annotation_marks_where_each_release_first_reported(self):
        # Releases change what is collected (ADR113 D5's disclosure gate makes
        # every machine start one run later), so the board marks each bex
        # release where it first reported. Builds with no release header are
        # not releases.
        [annotation] = [a for a in self.board["annotations"]["list"] if a["name"] == "Bex releases"]
        self.assertEqual(annotation["datasource"]["uid"], "cli-analytics")
        self.assertEqual(self.read_as_reader(annotation["target"]["rawSql"]), [
            {"time": "2026-08-17T12:00:00+00:00", "text": "bex 0.2.1", "tags": "release"},
            {"time": "2026-09-03T12:00:00+00:00", "text": "bex 0.2.2", "tags": "release"},
        ])

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
                                   "FROM cli_analytics.events WHERE received_at < '2026-10-01' "
                                   "GROUP BY 1, 2 ORDER BY 1")
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
        rows = self.psql("SELECT column_name || ':' || data_type FROM information_schema.columns "
                         "WHERE table_schema = 'cli_analytics' AND table_name = 'events' "
                         "ORDER BY ordinal_position").stdout.split("\n")
        names = [r.split(":")[0] for r in rows if r]
        self.assertEqual(names[:len(RELEASED_VIEW_COLUMNS)], RELEASED_VIEW_COLUMNS)
        # CREATE OR REPLACE VIEW also refuses a changed type, so the appended
        # generations are pinned with theirs: w5/m94, then ADR113 D7/D9 (w3/m439).
        self.assertEqual([r for r in rows if r][len(RELEASED_VIEW_COLUMNS):], [
            "bex_version:text", "exit_code:integer", "cli_fault:boolean",
            "command_family:text", "started_at:timestamp with time zone", "audience:text",
        ])

    def test_view_replaces_the_released_generation(self):
        # The bootstrap re-applies the SQL over whatever generation production
        # holds, and CREATE OR REPLACE VIEW refuses a renamed, reordered or
        # retyped column. Stand up the w5/m94 generation's exact shape and
        # apply the current SQL over it.
        released = RELEASED_VIEW_COLUMNS + ["bex_version"]
        types = ["timestamptz"] + ["text"] * 5 + ["boolean", "bigint"] + ["text"] * 5 + [
            "boolean", "text[]", "text[]", "text"]
        # An explicit length check, not zip(strict=True): that needs Python 3.10,
        # and macOS ships 3.9, where /ship's gate runs this suite.
        self.assertEqual(len(released), len(types))
        shape = ", ".join(f"NULL::{t} AS {c}" for c, t in zip(released, types))
        self.psql(f"DROP VIEW cli_analytics.events; CREATE VIEW cli_analytics.events AS SELECT {shape}")
        self.psql((ROOT / "scripts/cli-analytics.sql").read_text())
        self.assertEqual(self.read_as_reader("SELECT count(*) AS n FROM cli_analytics.events"), [{"n": 32}])

    def test_every_product_panel_carries_every_product_filter(self):
        # The predicates are threaded into each panel by hand; without this the
        # next panel added would silently ignore a selector. Audience split is
        # the one exception: it exists to show what the Audience filter removes.
        for panel in self.panels.values():
            sql = (panel.get("targets") or [{}])[0].get("rawSql") or ""
            if "${command:sqlstring}" in sql:
                with self.subTest(panel=panel["title"]):
                    for variable in ("version", "bexrelease", "output", "family"):
                        self.assertIn("${" + variable + ":sqlstring}", sql)
                    if panel["title"] == "Audience split":
                        self.assertNotIn("${audience:sqlstring}", sql)
                    else:
                        self.assertIn("${audience:sqlstring}", sql)

    def test_collection_health_ignores_product_filters(self):
        # These answer "is the collection working at all", so a product filter
        # must not be able to make a healthy collector look dead.
        for panel_id in (21, 37, 38, 39):
            sql = self.panels[panel_id]["targets"][0]["rawSql"]
            with self.subTest(panel=self.panels[panel_id]["title"]):
                self.assertNotIn("${", sql)

    def test_audience_filter_defaults_to_customer_and_unclassified(self):
        [audience] = [v for v in self.board["templating"]["list"] if v["name"] == "audience"]
        self.assertEqual(audience["current"]["value"], ["customer", "unclassified"])
        self.assertEqual(audience["query"].split(","), ["customer", "unclassified", "internal", "qa", "canary"])
        [family] = [v for v in self.board["templating"]["list"] if v["name"] == "family"]
        self.assertEqual(family["query"].split(","), ["hosting", "data", "agent", "automation", "coding-agent",
                                                       "desktop", "sandbox", "account", "other"])
        # 18 October events: two from the qa workspace, one from the internal one.
        self.assertEqual(self.october(2), [{"Observed commands": 15}])
        self.assertEqual(self.october(2, audience="qa"), [{"Observed commands": 2}])
        self.assertEqual(self.october(2, audience="__all"), [{"Observed commands": 18}])
        self.assertEqual(self.october(2, family=("agent", "automation")), [{"Observed commands": 5}])

    def scratch(self, rows, select):
        """Read select as the reader over rows that exist only inside a rolled-back transaction."""
        values = ", ".join(
            f"('scratch-{i}', 'scratch', 'tea-one', '2030-01-01+00', '{command}', '{kind}', "
            f"{duration}, {exit_code}, false, false)"
            for i, (command, kind, duration, exit_code) in enumerate(rows))
        result = self.psql(f"""BEGIN;
            INSERT INTO cli_telemetry_events (id, installation_id, workspace_id, received_at,
              command, completion_kind, duration_ms, exit_code, is_stdin_tty, is_stdout_tty)
            VALUES {values};
            SET LOCAL ROLE bex_cli_analytics;
            SELECT coalesce(json_agg(q), '[]'::json) FROM ({select} FROM cli_analytics.events
              WHERE received_at = '2030-01-01+00') q;
            ROLLBACK;""")
        return json.loads(result.stdout)

    def test_command_family_mapping(self):
        # Every command group of the pinned upstream tree and every Bex-native
        # one (bex --help, `bex agent --help`, `bex ea --help`), plus the early
        # access groups production has seen. Changing any row is a change to the
        # board's history: update docs/runbooks/cli-analytics.md with it.
        families = {
            "bex services": "hosting", "bex services create": "hosting", "bex deploys list": "hosting",
            "bex deploys create": "hosting", "bex jobs create": "hosting", "bex restart": "hosting",
            "bex logs": "hosting", "bex ssh": "hosting", "bex blueprints validate": "hosting",
            "bex ea blueprints": "hosting", "bex environments": "hosting", "bex projects": "hosting",
            "bex workflows tasks": "hosting",
            "bex postgres get": "data", "bex keyvalues list": "data", "bex psql": "data",
            "bex pgcli": "data", "bex kv-cli": "data", "bex ea objects put": "data",
            "bex s3": "data", "bex s3api": "data", "bex s3 keys list": "data", "bex s3 public": "data",
            "bex agent": "agent", "bex agent run": "agent", "bex agent wait": "agent",
            "bex agent secrets set": "agent", "bex agent skills": "agent", "bex agent ssh": "agent",
            "bex agent automations": "automation", "bex agent automations run": "automation",
            "bex code": "coding-agent", "bex code keys": "coding-agent", "bex glm": "coding-agent",
            "bex muse": "coding-agent", "bex kimi": "coding-agent", "bex deepseek": "coding-agent",
            "bex edit": "desktop", "bex agent open": "desktop",
            "bex sandboxes exec": "sandbox", "bex sandbox-groups create": "sandbox",
            "bex ea sandboxes": "sandbox", "bex ea sandbox-groups": "sandbox",
            "bex workspaces": "account", "bex workspace set": "account", "bex whoami": "account",
            "bex login": "account", "bex logout": "account", "bex upgrade": "account",
            "bex skills install": "account",
            "bex": "other", "bex docs": "other", "bex completion": "other", "bex help": "other",
            "bex analytics notice": "other", "bex ea": "other", "bex agentx": "other", "": "other",
            # An unmodified Render binary names itself render; the path after it decides.
            "render services create": "hosting", "render postgres": "data", "render": "other",
        }
        rows = self.scratch([(c, "success", 1, 0) for c in families], "SELECT command, command_family")
        got = {r["command"]: r["command_family"] for r in rows}
        self.assertEqual(got, {(c or "unknown"): f for c, f in families.items()})

    def test_cli_fault_truth_table(self):
        kinds = {
            "success": False, "explicit_exit": False, "discovery_error": True,
            "validation_error": True, "setup_error": True, "execution_error": True,
            "help": None, "version": None, "future_kind": None, "": None,
        }
        rows = self.scratch([("bex agent wait", k, 1, 3) for k in kinds],
                            "SELECT outcome, cli_fault, exit_code, failed")
        self.assertEqual({r["outcome"]: r["cli_fault"] for r in rows},
                         {(k or "unknown"): v for k, v in kinds.items()})
        # A failed agent turn reported through its exit code is still a failed
        # command, but not the CLI's fault.
        explicit = next(r for r in rows if r["outcome"] == "explicit_exit")
        self.assertEqual((explicit["exit_code"], explicit["failed"], explicit["cli_fault"]), (3, True, False))

    def test_agent_waits_are_session_stream(self):
        modes = {
            "bex agent chat": "session / stream", "bex agent run": "session / stream",
            "bex agent wait": "session / stream", "bex agent send": "session / stream",
            "bex agent logs": "session / stream", "bex agent handoff": "session / stream",
            "bex logs": "session / stream", "bex psql": "session / stream", "bex ssh": "session / stream",
            "bex agent list": "non-interactive", "bex agent automations run": "non-interactive",
            "bex services": "non-interactive",
        }
        rows = self.scratch([(c, "success", 3600000, 0) for c in modes], "SELECT command, duration_mode")
        self.assertEqual({r["command"]: r["duration_mode"] for r in rows}, modes)

    def test_audience_join_defaults_to_unclassified(self):
        rows = self.read_as_reader("SELECT DISTINCT coalesce(workspace_id, '(none)') AS workspace, audience "
                                   "FROM cli_analytics.events ORDER BY 1")
        self.assertEqual({r["workspace"]: r["audience"] for r in rows}, {
            "(none)": "unclassified", "tea-cust": "customer", "tea-int": "internal",
            "tea-one": "unclassified", "tea-qa": "qa",
        })

    def test_reader_selects_the_new_columns(self):
        rows = self.read_as_reader("SELECT exit_code, cli_fault, command_family, started_at, audience "
                                   "FROM cli_analytics.events WHERE received_at = '2026-10-05 09:00+00'")
        self.assertEqual(rows, [{"exit_code": 0, "cli_fault": False, "command_family": "hosting",
                                 "started_at": "2026-10-05T08:59:57+00:00", "audience": "customer"}])

    def test_audience_split_ignores_the_audience_filter(self):
        split = {r["Audience"]: r for r in self.october(29)}
        self.assertEqual({a: r["Commands"] for a, r in split.items()},
                         {"customer": 14, "qa": 2, "unclassified": 1, "internal": 1})
        self.assertEqual(split["customer"]["Installations"], 5)
        self.assertEqual(split["unclassified"]["Workspaces"], 0)
        self.assertAlmostEqual(split["qa"]["Share of commands"], 2 / 18)

    def test_command_families_reach_and_volume(self):
        families = {r["Family"]: r for r in self.october(30)}
        self.assertEqual({f: r["Commands"] for f, r in families.items()}, {
            "hosting": 5, "agent": 4, "data": 2, "account": 1, "automation": 1,
            "coding-agent": 1, "desktop": 1,
        })
        self.assertEqual(families["hosting"]["Installations"], 3)
        self.assertEqual(families["hosting"]["Distinct commands"], 4)
        reach = {r["Family"]: r["Share of active installations"] for r in self.october(31)}
        self.assertEqual(reach["hosting"], 0.5)
        self.assertAlmostEqual(reach["data"], 2 / 6)
        self.assertAlmostEqual(reach["agent"], 1 / 6)
        # Shares overlap: an installation counts once in every family it used.
        self.assertGreater(sum(reach.values()), 1)

    def test_agent_driven_share_by_family(self):
        rows = {r["Family"]: r for r in self.october(32)}
        self.assertEqual(rows["agent"]["Agent detected"], 3)
        self.assertEqual(rows["agent"]["Agent-driven share"], 0.75)
        self.assertEqual(rows["agent"]["Share of agent commands"], 1)
        self.assertEqual(rows["hosting"]["Agent-driven share"], 0)
        self.assertEqual(rows["hosting"]["Share of agent commands"], 0)

    def test_cli_faults_and_reported_failures_split_the_failure_share(self):
        days = {r["time"][5:10]: r for r in self.october(33)}
        self.assertEqual((days["10-05"]["CLI fault"], days["10-05"]["Reported failure"]), (0.5, 0))
        # A failed turn and a timed-out wait: the CLI worked, the agent did not.
        self.assertEqual((days["10-06"]["CLI fault"], days["10-06"]["Reported failure"]), (0, 1))
        self.assertAlmostEqual(days["10-07"]["Reported failure"], 1 / 3)
        # Only the internal workspace was active that day.
        self.assertEqual((days["10-09"]["CLI fault"], days["10-09"]["Reported failure"]), (None, None))

    def test_agent_outcomes_by_exit_code(self):
        rows = {r["Exit code"]: (r["Meaning"], r["Commands"]) for r in self.october(34)}
        # The help row is no outcome, and agent open is the desktop family.
        self.assertEqual(rows, {0: ("succeeded", 1), 3: ("turn failed", 1), 6: ("automation started no session", 1),
                                124: ("timed out", 1)})

    def test_activation_counts_a_successful_create_or_run_within_seven_days(self):
        # Week of 10-05: n1 (services create at first sight), n2 (agent run six
        # days after a failed one) activate; n3's deploy came on day eight; n5
        # and the render binary n6 never created anything. 10-12 is immature.
        self.assertEqual(self.october(35), [{"Cohort (UTC)": "2026-10-05T00:00:00+00:00", "Installations": 5,
                                             "Activated within 7 days": 2, "Activation": 0.4}])
        self.assertEqual(self.october(35, audience="qa")[0]["Activation"], 1)
        self.assertEqual(self.query(35, start="2026-10-05T00:00:00Z", end="2026-10-18T00:00:00Z"), [])

    def test_unmodified_render_binaries(self):
        self.assertEqual(self.october(36), [{"Upstream version": "2.30.0", "Installations": 1, "Workspaces": 1,
                                             "Commands": 2, "Last seen": "2026-10-19T18:00:00+00:00"}])
        # A pre-header bex build also reports no release, but names itself bex.
        self.assertEqual(self.query(36, start="2026-09-01T00:00:00Z", end="2026-09-30T00:00:00Z"), [])

    def test_delivery_lag_excludes_the_command_run_time(self):
        days = {r["time"][5:10]: r for r in self.october(37)}
        # An hour-long agent wait received a second after it ended lags one second.
        self.assertAlmostEqual(days["10-06"]["p50"], 1.5)
        self.assertAlmostEqual(days["10-06"]["p95"], 1.95)
        # A client clock ahead of the server shows as negative lag, not as zero.
        self.assertAlmostEqual(days["10-08"]["p50"], -5.1)
        self.assertEqual(days["10-09"]["p50"], None)

    def test_retained_window_and_clients_by_release(self):
        [window] = self.query(38)
        self.assertEqual(window["Retained events"], 32)
        # Oldest fixture: 2026-08-17 12:00, 63.5 days before the stand-in now.
        self.assertEqual(window["Oldest retained event"], 63.5 * 86400)
        clients = self.query(39)
        self.assertEqual([(r["Bex release"], r["Events"]) for r in clients], [("0.2.2", 1), ("unknown", 1)])

    def test_reader_cannot_read_product_tables_or_mutate(self):
        for sql in (
            "SELECT * FROM private_accounts", "SELECT * FROM cli_telemetry_events",
            "SELECT * FROM product_analytics_audiences",
            "DELETE FROM cli_analytics.events", "DROP VIEW cli_analytics.events",
            "CREATE TABLE cli_analytics.surprise (id int)",
        ):
            with self.subTest(sql=sql):
                result = self.psql("SET ROLE bex_cli_analytics; " + sql, check=False)
                self.assertNotEqual(result.returncode, 0)
                # Since the audience join the view is no longer updatable for
                # anyone, so a write is refused before its permission is checked.
                self.assertRegex(result.stderr, "permission denied|must be owner|cannot delete from view")
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


# Ratio axes that pass 100% by design: requests and usage over allocatable
# overcommit a node, so a cap at 1 would clip exactly the excursion they exist
# to show.
MAY_EXCEED_FULL = {
    ("bex-cluster-capacity", "CPU commitment by node (requests / allocatable)"),
    ("bex-cluster-capacity", "Saturation vs allocatable by node"),
}


class RatioAxisTest(unittest.TestCase):
    """Every board, not just cli-usage, and no database needed (w3/072)."""

    def test_every_ratio_axis_has_a_top(self):
        # When every point of a percentunit series is 0, Grafana's degenerate
        # range fallback is 0-100 raw, which percentunit draws as 0-10000%: a
        # quiet ratio reads as a broken one. A hard "max" or a soft
        # custom.axisSoftMax pins the top; overrides count, since a series that
        # takes percentunit from one gets its own axis.
        values = (ROOT / "deploy/gitops/base/values/grafana.values.yaml").read_text()
        boards = [json.loads(m.group(1)) for m in
                  re.finditer(r"^      json: \|\n((?:        .*\n|\n)+)", values, re.M)]
        self.assertEqual(len(boards), values.count("json: |"))

        def walk(panels):
            for panel in panels:
                yield panel
                yield from walk(panel.get("panels", []))

        unbounded = set()
        for board in boards:
            for panel in walk(board["panels"]):
                if panel["type"] != "timeseries":
                    continue
                config = panel.get("fieldConfig", {})
                defaults = config.get("defaults", {})
                base = {"unit": defaults.get("unit"), "max": defaults.get("max"),
                        "custom.axisSoftMax": defaults.get("custom", {}).get("axisSoftMax")}
                fields = [base] + [{**base, **{p["id"]: p["value"] for p in o["properties"]}}
                                   for o in config.get("overrides", [])]
                if any(f["unit"] == "percentunit" and f["max"] is None
                       and f["custom.axisSoftMax"] is None for f in fields):
                    unbounded.add((board["uid"], panel["title"]))
        self.assertEqual(unbounded, MAY_EXCEED_FULL)


if __name__ == "__main__":
    unittest.main()

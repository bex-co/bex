"""Offline mode guards for m53 billing operations scripts."""

import os
import json
import stat
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
FIXTURES = ROOT / "scripts" / "stripe-billing-prod-test-fixtures.sh"
RECONCILE = ROOT / "scripts" / "stripe-billing-reconcile.sh"


class BillingOperationsGuardTest(unittest.TestCase):
    def fixture_plan(self, key):
        env = os.environ.copy()
        env.update(
            {
                "BEX_STRIPE_SECRET_KEY": key,
                "BEX_CP_TOKEN": "offline",
                "BEX_BILLING_FIXTURE_DB_URI": "postgres://offline:offline@localhost/offline",
            }
        )
        return subprocess.run(
            ["bash", str(FIXTURES), "plan"],
            cwd=ROOT,
            env=env,
            text=True,
            capture_output=True,
            check=False,
        )

    def test_fixture_plan_accepts_test_and_refuses_live(self):
        test = self.fixture_plan("rk_test_offline")
        self.assertEqual(0, test.returncode, test.stderr)
        self.assertIn("DRY RUN", test.stdout)
        live = self.fixture_plan("rk_live_offline")
        self.assertNotEqual(0, live.returncode)
        self.assertIn("live Stripe keys are refused", live.stderr)

    def reconcile_report(self, extra_env):
        env = os.environ.copy()
        env.update({"BEX_CP_TOKEN": "offline"})
        env.update(extra_env)
        return subprocess.run(
            [
                "bash",
                str(RECONCILE),
                "report",
                "tea-offline",
                "2026-07-01T00:00:00Z",
                "2026-07-02T00:00:00Z",
            ],
            cwd=ROOT,
            env=env,
            text=True,
            capture_output=True,
            check=False,
        )

    def test_reconcile_refuses_live_before_provider_call(self):
        # w4/m81 t005: the read-only report admits a live key only under the
        # explicit BEX_STRIPE_ALLOW_LIVE=1 go-live decision; without it the
        # key gate still fails before any provider call.
        result = self.reconcile_report({"BEX_STRIPE_SECRET_KEY": "sk_live_offline"})
        self.assertNotEqual(0, result.returncode)
        self.assertIn("BEX_STRIPE_ALLOW_LIVE=1", result.stderr)

    def test_reconcile_repair_refuses_live_key_even_with_opt_in(self):
        # repair is the mutating path: BEX_STRIPE_ALLOW_LIVE never admits it.
        env = os.environ.copy()
        env.update(
            {
                "BEX_STRIPE_SECRET_KEY": "sk_live_offline",
                "BEX_STRIPE_ALLOW_LIVE": "1",
                "BEX_CP_TOKEN": "offline",
            }
        )
        result = subprocess.run(
            [
                "bash",
                str(RECONCILE),
                "repair",
                "tx-offline",
                "acknowledge",
                "ops",
                "offline test",
            ],
            cwd=ROOT,
            env=env,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertNotEqual(0, result.returncode)
        self.assertIn("live Stripe keys are refused for mutating", result.stderr)

    def test_reconcile_report_admits_live_key_under_explicit_opt_in(self):
        # With the flag, the key gate passes and the script proceeds to the
        # next requirement (the stripe CLI / control-plane fetch) — proving
        # the refusal above was the gate, not an environment accident.
        env = {
            "BEX_STRIPE_SECRET_KEY": "sk_live_offline",
            "BEX_STRIPE_ALLOW_LIVE": "1",
            # Point curl at a dead local port so the run still fails offline,
            # but past the key gate.
            "BEX_CP_URL": "http://127.0.0.1:9",
        }
        result = self.reconcile_report(env)
        self.assertNotEqual(0, result.returncode)
        self.assertNotIn("BEX_STRIPE_ALLOW_LIVE", result.stderr)
        self.assertNotIn("live Stripe keys", result.stderr)

    def test_partial_fixture_state_can_be_cleaned_without_provider_mutation(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bin_dir = root / "bin"
            bin_dir.mkdir()
            for name in ("psql", "stripe"):
                command = bin_dir / name
                command.write_text("#!/usr/bin/env bash\nexit 99\n", encoding="utf-8")
                command.chmod(command.stat().st_mode | stat.S_IXUSR)
            state = root / "partial.json"
            state.write_text(
                json.dumps(
                    {
                        "windowStart": "2026-07-27T00:00:00Z",
                        "workspaces": {},
                    }
                ),
                encoding="utf-8",
            )
            state.chmod(0o600)
            env = os.environ.copy()
            env.update(
                {
                    "PATH": f"{bin_dir}:{env['PATH']}",
                    "BEX_STRIPE_SECRET_KEY": "rk_test_offline",
                    "BEX_CP_TOKEN": "offline",
                    "BEX_BILLING_FIXTURE_DB_URI": "postgres://offline:offline@localhost/offline",
                    "BEX_BILLING_FIXTURE_STATE": str(state),
                }
            )
            result = subprocess.run(
                ["bash", str(FIXTURES), "cleanup"],
                cwd=ROOT,
                env=env,
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(0, result.returncode, result.stderr)
            self.assertFalse(state.exists())
            self.assertIn("deleted exact", result.stdout)


if __name__ == "__main__":
    unittest.main()

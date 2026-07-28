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

    def test_reconcile_refuses_live_before_provider_call(self):
        env = os.environ.copy()
        env.update(
            {
                "BEX_STRIPE_SECRET_KEY": "sk_live_offline",
                "BEX_CP_TOKEN": "offline",
            }
        )
        result = subprocess.run(
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
        self.assertNotEqual(0, result.returncode)
        self.assertIn("live Stripe keys are refused", result.stderr)

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

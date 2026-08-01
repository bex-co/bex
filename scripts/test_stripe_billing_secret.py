"""Offline contract tests for scripts/stripe-billing-secret.sh."""

import os
import subprocess
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("stripe-billing-secret.sh")


class StripeBillingSecretTest(unittest.TestCase):
    def run_secret(self, reconcile, require_payment_method=None):
        env = os.environ.copy()
        env.update(
            {
                "BEX_STRIPE_ENV_FILE": "/dev/null",
                "BEX_STRIPE_SECRET_KEY": "rk_test_offline_fixture",
                "BEX_STRIPE_WEBHOOK_SECRET": "whsec_offline_fixture",
                "BEX_STRIPE_EPOCH": "2026-07-01T00:00:00Z",
                "BEX_STRIPE_DUNNING_ENABLED": "1",
                "BEX_STRIPE_GRACE_PERIOD": "168h",
                "BEX_STRIPE_RECONCILE_INTERVAL": reconcile,
                "DRY_RUN": "1",
            }
        )
        if require_payment_method is not None:
            env["BEX_REQUIRE_PAYMENT_METHOD"] = require_payment_method
        return subprocess.run(
            ["bash", str(SCRIPT)],
            capture_output=True,
            text=True,
            env=env,
            check=False,
        )

    def test_reconcile_interval_rejects_less_than_one_minute(self):
        result = self.run_secret("59.999s")
        self.assertNotEqual(0, result.returncode)
        self.assertIn("BEX_STRIPE_RECONCILE_INTERVAL must be >= 1m", result.stderr)

    def test_reconcile_interval_accepts_one_minute_or_composite_duration(self):
        for value in ("1m", "1m30s"):
            with self.subTest(value=value):
                result = self.run_secret(value)
                self.assertEqual(0, result.returncode, result.stderr)
                self.assertIn(f"reconcile={value}", result.stdout)
                self.assertNotIn("rk_test_offline_fixture", result.stdout + result.stderr)
                self.assertNotIn("whsec_offline_fixture", result.stdout + result.stderr)

    def test_payment_gate_defaults_off_and_accepts_explicit_opt_in(self):
        default = self.run_secret("1m")
        self.assertEqual(0, default.returncode, default.stderr)
        self.assertIn("payment_gate=0", default.stdout)
        self.assertIn("BEX_REQUIRE_PAYMENT_METHOD", default.stdout)

        enabled = self.run_secret("1m", "1")
        self.assertEqual(0, enabled.returncode, enabled.stderr)
        self.assertIn("payment_gate=1", enabled.stdout)

    def test_payment_gate_rejects_unknown_value(self):
        result = self.run_secret("1m", "true")
        self.assertNotEqual(0, result.returncode)
        self.assertIn("BEX_REQUIRE_PAYMENT_METHOD must be 0 or 1", result.stderr)

    def test_live_key_is_refused_without_separate_opt_in(self):
        env = os.environ.copy()
        env.update(
            {
                "BEX_STRIPE_ENV_FILE": "/dev/null",
                "BEX_STRIPE_SECRET_KEY": "rk_live_offline_fixture",
                "BEX_STRIPE_WEBHOOK_SECRET": "whsec_offline_fixture",
                "BEX_STRIPE_EPOCH": "2026-07-01T00:00:00Z",
                "DRY_RUN": "1",
            }
        )
        result = subprocess.run(
            ["bash", str(SCRIPT)], capture_output=True, text=True, env=env, check=False
        )
        self.assertNotEqual(0, result.returncode)
        self.assertIn("live restricted keys are refused by default", result.stderr)


if __name__ == "__main__":
    unittest.main()

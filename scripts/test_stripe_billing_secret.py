"""Offline contract tests for scripts/stripe-billing-secret.sh."""

import os
import subprocess
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("stripe-billing-secret.sh")


class StripeBillingSecretTest(unittest.TestCase):
    def run_secret(self, reconcile):
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


if __name__ == "__main__":
    unittest.main()

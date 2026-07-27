"""Offline contract tests for scripts/stripe-billing-setup.py."""

import importlib.util
import unittest
from decimal import Decimal
from pathlib import Path
from unittest import mock


SCRIPT = Path(__file__).with_name("stripe-billing-setup.py")
SPEC = importlib.util.spec_from_file_location("stripe_billing_setup", SCRIPT)
SETUP = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SETUP)


class StripeBillingSetupTest(unittest.TestCase):
    def test_list_all_follows_cursor(self):
        with mock.patch.object(
            SETUP,
            "stripe",
            side_effect=[
                {"data": [{"id": "one"}], "has_more": True},
                {"data": [{"id": "two"}], "has_more": False},
            ],
        ) as stripe:
            self.assertEqual([{"id": "one"}, {"id": "two"}], SETUP.list_all("coupons"))
            self.assertEqual(
                mock.call("coupons", "list", "--limit", "100", "-d", "starting_after=one"),
                stripe.call_args_list[1],
            )

    def test_pricing_catalog_has_expected_paid_dimensions(self):
        dimensions = SETUP.parse_pricing()
        names = {name for name, _, _ in dimensions}
        self.assertEqual(13, len(dimensions))
        self.assertIn("instance_seconds.service.starter", names)
        self.assertIn("instance_seconds.postgres.basic-1gb", names)
        self.assertIn("instance_seconds.key_value.standard", names)
        self.assertIn("egress_gib", names)
        self.assertIn("build_seconds", names)
        self.assertIn("storage_gb_hours", names)
        self.assertFalse(any(name.endswith(".free") for name in names))

    def test_existing_matching_price_is_reused(self):
        price = {
            "id": "price_1",
            "currency": "usd",
            "type": "recurring",
            "unit_amount_decimal": "1.5",
            "recurring": {
                "interval": "month",
                "usage_type": "metered",
                "meter": "mtr_1",
            },
        }
        with mock.patch.object(SETUP, "existing_price", return_value=price), \
                mock.patch.object(SETUP, "stripe") as stripe:
            self.assertEqual(
                "exists",
                SETUP.ensure_price("egress_gib", "egress", Decimal("1.5"), "mtr_1"),
            )
            stripe.assert_not_called()

    def test_existing_mismatched_price_fails_closed(self):
        price = {
            "id": "price_stale",
            "currency": "usd",
            "type": "recurring",
            "unit_amount_decimal": "2",
            "recurring": {
                "interval": "month",
                "usage_type": "metered",
                "meter": "mtr_1",
            },
        }
        with mock.patch.object(SETUP, "existing_price", return_value=price):
            with self.assertRaisesRegex(RuntimeError, "does not match"):
                SETUP.ensure_price("egress_gib", "egress", Decimal("1.5"), "mtr_1")

    def test_comp_coupon_is_idempotent_and_validated(self):
        coupon = {
            "id": SETUP.COMP_COUPON_ID,
            "valid": True,
            "percent_off": 100.0,
            "duration": "forever",
        }
        with mock.patch.object(SETUP, "stripe") as stripe:
            self.assertEqual(
                "exists", SETUP.ensure_comp_coupon({SETUP.COMP_COUPON_ID: coupon})
            )
            stripe.assert_not_called()

        coupon["percent_off"] = 50
        with self.assertRaisesRegex(RuntimeError, "not a valid"):
            SETUP.ensure_comp_coupon({SETUP.COMP_COUPON_ID: coupon})

    def test_comp_coupon_is_created_once(self):
        coupons = {}
        with mock.patch.object(
            SETUP,
            "stripe",
            return_value={
                "id": SETUP.COMP_COUPON_ID,
                "valid": True,
                "percent_off": 100.0,
                "duration": "forever",
            },
        ) as stripe:
            self.assertEqual("created", SETUP.ensure_comp_coupon(coupons))
            self.assertIn(SETUP.COMP_COUPON_ID, coupons)
            stripe.assert_called_once()


if __name__ == "__main__":
    unittest.main()

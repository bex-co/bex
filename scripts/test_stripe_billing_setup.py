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
    def test_stripe_rejects_api_error_json_even_when_cli_exits_zero(self):
        result = mock.Mock(returncode=0, stdout='{"error":{"message":"denied"}}', stderr="")
        with mock.patch.object(SETUP.subprocess, "run", return_value=result):
            with self.assertRaisesRegex(RuntimeError, "denied"):
                SETUP.stripe("post", "/anything")

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
        self.assertEqual(14, len(dimensions))
        self.assertIn("instance_seconds.service.starter", names)
        self.assertIn("instance_seconds.postgres.basic-1gb", names)
        self.assertIn("instance_seconds.key_value.standard", names)
        self.assertIn("egress_gib", names)
        self.assertIn("build_seconds", names)
        self.assertIn("storage_gb_hours", names)
        self.assertIn("sandbox_compute_seconds", names)
        self.assertFalse(any(name.endswith(".free") for name in names))

    def test_pricing_catalog_rates_match_published_units(self):
        dimensions = {name: cents for name, _, cents in SETUP.parse_pricing()}
        month_seconds = Decimal(730 * 60 * 60)
        monthly = {
            "instance_seconds.service.starter": "4.90",
            "instance_seconds.service.standard": "17.50",
            "instance_seconds.service.pro": "59.50",
            "instance_seconds.service.pro-plus": "122.50",
            "instance_seconds.service.pro-max": "157.50",
            "instance_seconds.service.pro-ultra": "315.00",
            "instance_seconds.postgres.basic-256mb": "4.90",
            "instance_seconds.postgres.basic-1gb": "14.00",
            "instance_seconds.key_value.starter": "7.00",
            "instance_seconds.key_value.standard": "21.00",
        }
        for name, want in monthly.items():
            with self.subTest(name=name):
                dollars = dimensions[name] * month_seconds / Decimal(100)
                self.assertEqual(Decimal(want), dollars.quantize(Decimal("0.01")))

        self.assertEqual(Decimal("1.5"), dimensions["egress_gib"])
        build_dollars_per_minute = dimensions["build_seconds"] * Decimal(60) / Decimal(100)
        self.assertEqual(
            Decimal("0.0035"),
            build_dollars_per_minute.quantize(Decimal("0.0001")),
        )
        storage_dollars_per_month = dimensions["storage_gb_hours"] * Decimal(730) / Decimal(100)
        self.assertEqual(
            Decimal("0.21"),
            storage_dollars_per_month.quantize(Decimal("0.01")),
        )
        sandbox_dollars_per_vcpu_hour = (
            dimensions["sandbox_compute_seconds"] * Decimal(1000 * 3600) / Decimal(100)
        )
        self.assertEqual(
            Decimal("0.0895"),
            sandbox_dollars_per_vcpu_hour.quantize(Decimal("0.0001")),
        )

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

    def test_tax_gate_requires_both_inputs_and_a_matching_test_registration(self):
        with self.assertRaisesRegex(RuntimeError, "supplied together"):
            SETUP.validate_tax_gate("txcd_10000000", None, False)

        with mock.patch.object(
            SETUP,
            "stripe",
            return_value={"data": [{"id": "taxreg_live", "livemode": True}]},
        ):
            with self.assertRaisesRegex(RuntimeError, "no active test-mode"):
                SETUP.validate_tax_gate("txcd_10000000", "exclusive", False)

        with mock.patch.object(
            SETUP,
            "stripe",
            return_value={"data": [{"id": "taxreg_test", "livemode": False}]},
        ):
            self.assertEqual(
                1,
                SETUP.validate_tax_gate("txcd_10000000", "exclusive", False),
            )

    def test_portal_configuration_is_scoped_and_idempotent(self):
        with mock.patch.object(SETUP, "list_all", return_value=[]), mock.patch.object(
            SETUP,
            "stripe",
            return_value={"id": "bpc_test"},
        ) as stripe:
            self.assertEqual(
                ("bpc_test", "created"),
                SETUP.ensure_portal_configuration("https://dashboard.bex.co/"),
            )
            args = stripe.call_args.args
            self.assertEqual(("billing_portal", "configurations", "create"), args[:3])
            self.assertIn("default_return_url=https://dashboard.bex.co/usage", args)
            self.assertIn("features[payment_method_update][enabled]=true", args)
            self.assertIn("features[subscription_cancel][enabled]=false", args)

        existing = {"id": "bpc_test", "metadata": {SETUP.PORTAL_METADATA_KEY: "true"}}
        with mock.patch.object(SETUP, "list_all", return_value=[existing]), mock.patch.object(
            SETUP,
            "stripe",
            return_value={"id": "bpc_test"},
        ) as stripe:
            self.assertEqual(
                ("bpc_test", "exists"),
                SETUP.ensure_portal_configuration("https://dashboard.bex.co"),
            )
            self.assertEqual(
                ("billing_portal", "configurations", "update", "bpc_test"),
                stripe.call_args.args[:4],
            )

    def test_portal_configuration_rejects_non_origin_urls(self):
        for url in (
            "http://dashboard.bex.co",
            "https://dashboard.bex.co/usage",
            "https://user@dashboard.bex.co",
            "https://dashboard.bex.co?next=elsewhere",
        ):
            with self.subTest(url=url), self.assertRaisesRegex(RuntimeError, "HTTPS origin"):
                SETUP.ensure_portal_configuration(url)


if __name__ == "__main__":
    unittest.main()

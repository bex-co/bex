"""Offline contract tests for Stripe billing reconciliation."""

import importlib.util
import unittest
from pathlib import Path
from unittest import mock


MODULE = Path(__file__).with_name("stripe_billing_reconcile.py")
SPEC = importlib.util.spec_from_file_location("stripe_billing_reconcile", MODULE)
RECONCILE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
SPEC.loader.exec_module(RECONCILE)


def row(kind, quantity, event, state="emitted", transaction="tx-1"):
    return {
        "kind": kind,
        "quantity": quantity,
        "eventName": event,
        "state": state,
        "transactionId": transaction,
    }


class ReconcileTest(unittest.TestCase):
    def test_normalization_matches_stripe_twelve_decimal_payload(self):
        self.assertEqual(
            "1.423845122568",
            str(RECONCILE.normalized_value(row("egress_bytes", 1528842059, "egress_gib"))),
        )
        self.assertEqual(
            "0.000277777778",
            str(RECONCILE.normalized_value(row("storage_gb_seconds", 1, "storage_gb_hours"))),
        )

    def test_invoice_preview_fetches_every_paginated_line(self):
        first = [{"id": f"il_{index}"} for index in range(100)]
        second = [{"id": f"il_{index}"} for index in range(100, 113)]
        responses = [
            {
                "id": "upcoming_in_test",
                "lines": {
                    "data": first[:10],
                    "has_more": True,
                    "url": "/v1/invoices/upcoming_in_test/lines",
                },
            },
            {"data": first, "has_more": True},
            {"data": second, "has_more": False},
        ]
        with mock.patch.object(RECONCILE, "stripe_json", side_effect=responses) as stripe_json:
            lines = RECONCILE.invoice_preview_lines("cus_test", "sub_test")
        self.assertEqual(113, len(lines))
        self.assertEqual("il_112", lines[-1]["id"])
        self.assertIn("starting_after=il_99", stripe_json.call_args_list[-1].args[0])

    def test_invoice_preview_rejects_stalled_pagination(self):
        responses = [
            {"lines": {"url": "/v1/invoices/upcoming_in_test/lines"}},
            {"data": [], "has_more": True},
        ]
        with mock.patch.object(RECONCILE, "stripe_json", side_effect=responses):
            with self.assertRaisesRegex(RuntimeError, "did not advance"):
                RECONCILE.invoice_preview_lines("cus_test", "sub_test")

    def test_matches_every_normalized_dimension(self):
        local = {
            "workspaceId": "tea-test",
            "livemode": False,
            "rows": [
                row("instance_seconds", 3600, "instance_seconds.service.starter", transaction="tx-1"),
                row("egress_bytes", 1073741824, "egress_gib", transaction="tx-2"),
                row("storage_gb_seconds", 1800, "storage_gb_hours", transaction="tx-3"),
                row("build_seconds", 30, "build_seconds", transaction="tx-4"),
            ],
        }
        result = RECONCILE.compare(
            local,
            {
                "instance_seconds.service.starter": "3600",
                "egress_gib": "1.0",
                "storage_gb_hours": "0.5",
                "build_seconds": "30",
            },
            [
                {"lookupKey": "instance_seconds.service.starter", "quantity": 3600, "amount": 36, "currency": "usd", "unitAmountDecimal": "0.01"},
                {"lookupKey": "egress_gib", "quantity": 1, "amount": 9, "currency": "usd", "unitAmountDecimal": "9"},
                {"lookupKey": "storage_gb_hours", "quantity": 0, "amount": 1, "currency": "usd", "unitAmountDecimal": "1.5"},
                {"lookupKey": "build_seconds", "quantity": 30, "amount": 3, "currency": "usd", "unitAmountDecimal": "0.1"},
            ],
        )
        self.assertTrue(result["ok"], result)

    def test_missing_and_rounding_mismatch_fail(self):
        local = {
            "workspaceId": "tea-test",
            "livemode": False,
            "rows": [row("egress_bytes", 1, "egress_gib")],
        }
        result = RECONCILE.compare(local, {"egress_gib": "0.000000001"}, [])
        self.assertFalse(result["ok"])
        kinds = {problem["type"] for problem in result["problems"]}
        self.assertEqual({"quantity_mismatch", "missing_invoice_line"}, kinds)

    def test_rejected_ambiguous_and_duplicates_fail(self):
        local = {
            "workspaceId": "tea-test",
            "livemode": False,
            "rows": [
                row("build_seconds", 1, "build_seconds", "rejected", "same"),
                row("build_seconds", 1, "build_seconds", "ambiguous", "same"),
            ],
        }
        result = RECONCILE.compare(
            local,
            {},
            [
                {"lookupKey": "build_seconds", "quantity": 1, "amount": 1, "currency": "usd"},
                {"lookupKey": "build_seconds", "quantity": 1, "amount": 1, "currency": "usd"},
            ],
        )
        kinds = {problem["type"] for problem in result["problems"]}
        self.assertEqual(
            {"rejected", "ambiguous", "duplicate_local_transaction", "duplicate_invoice_line"},
            kinds,
        )

    def test_invoice_quantity_and_amount_are_reconciled(self):
        local = {
            "workspaceId": "tea-test",
            "livemode": False,
            "rows": [row("build_seconds", 30, "build_seconds")],
        }
        result = RECONCILE.compare(
            local,
            {"build_seconds": "30"},
            [{"lookupKey": "build_seconds", "quantity": 29, "amount": None, "currency": "", "unitAmountDecimal": "0.1"}],
        )
        kinds = {problem["type"] for problem in result["problems"]}
        self.assertEqual({"invoice_quantity_mismatch", "invoice_amount_missing"}, kinds)


if __name__ == "__main__":
    unittest.main()

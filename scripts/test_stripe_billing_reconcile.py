"""Offline contract tests for Stripe billing reconciliation."""

import importlib.util
import unittest
from pathlib import Path


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
                {"lookupKey": "instance_seconds.service.starter", "quantity": 3600, "amount": 36, "currency": "usd"},
                {"lookupKey": "egress_gib", "quantity": 1, "amount": 9, "currency": "usd"},
                {"lookupKey": "storage_gb_hours", "quantity": "0.5", "amount": 1, "currency": "usd"},
                {"lookupKey": "build_seconds", "quantity": 30, "amount": 3, "currency": "usd"},
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
            [{"lookupKey": "build_seconds", "quantity": 29, "amount": None, "currency": ""}],
        )
        kinds = {problem["type"] for problem in result["problems"]}
        self.assertEqual({"invoice_quantity_mismatch", "invoice_amount_missing"}, kinds)


if __name__ == "__main__":
    unittest.main()

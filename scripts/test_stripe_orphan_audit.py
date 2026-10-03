import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from stripe_orphan_audit import classify  # noqa: E402


def customer(cid="cus_1", workspace="tea-a", **meta):
    return {"id": cid, "metadata": {"bex_workspace": workspace, **meta} if workspace else dict(meta)}


class ClassifyTest(unittest.TestCase):
    def mapping(self, **kw):
        row = {"workspaceId": "tea-a", "customerId": "cus_1", "checkoutStarted": False, "comped": False}
        row.update(kw)
        return {"tea-a": row}

    def test_payment_method_wins_over_everything(self):
        c = customer(bex_deleted_at="x")
        c["invoice_settings"] = {"default_payment_method": "pm_1"}
        self.assertEqual(classify(c, False, None), "bound")
        self.assertEqual(classify(customer(), True, None), "bound")

    def test_intended_cardless_owners_are_not_reclaim_targets(self):
        self.assertEqual(classify(customer(bex_deleted_at="2026-09-01"), False, None), "tombstone")
        self.assertEqual(classify(customer(bex_workspace_creation_attempt="wca-1"), False, None), "workspace_create")
        self.assertEqual(classify(customer(), False, self.mapping(comped=True)), "comped")

    def test_unbound_buckets_follow_the_local_mapping(self):
        self.assertEqual(classify(customer(), False, self.mapping(checkoutStarted=True)), "unbound_checkout")
        self.assertEqual(classify(customer(), False, self.mapping()), "unbound_no_checkout")
        self.assertEqual(classify(customer(), False, {}), "unbound_unmapped")
        # A mapping that names a different Customer means this one is a stray duplicate.
        self.assertEqual(classify(customer(cid="cus_2"), False, self.mapping()), "unbound_unmapped")
        self.assertEqual(classify(customer(), False, None), "unbound_unknown")

    def test_untagged_customers_are_flagged_for_review(self):
        self.assertEqual(classify(customer(workspace=""), False, self.mapping()), "untagged")


if __name__ == "__main__":
    unittest.main()

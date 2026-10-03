#!/usr/bin/env python3
"""Read-only audit of cardless bex Stripe Customers.

Buckets every Stripe Customer by the code path that minted it, so the
abandoned-checkout reclaimer's effect can be measured before and after it
runs. Never writes to Stripe or the database and never prints names, emails,
or payment details — only Customer ids, workspace ids, and counts.

Usage:
  BEX_STRIPE_SECRET_KEY=rk_test_… scripts/stripe_orphan_audit.py [--mappings FILE]

A live key additionally requires BEX_STRIPE_ALLOW_LIVE=1. FILE is the JSON
array printed by MAPPINGS_SQL below (run it read-only against the control-plane
database, e.g. `psql "$BEX_CP_DB_URI" -At -c "$(scripts/stripe_orphan_audit.py --print-sql)"`);
without it, tagged unbound Customers are reported as `unbound_unknown`.
"""

from __future__ import annotations

import argparse
import json
import sys
from collections import Counter
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from stripe_billing_reconcile import stripe_json  # noqa: E402

WORKSPACE_KEY = "bex_workspace"
DELETED_KEY = "bex_deleted_at"
ATTEMPT_KEY = "bex_workspace_creation_attempt"

MAPPINGS_SQL = """SELECT COALESCE(json_agg(json_build_object(
  'workspaceId', m.workspace_id, 'customerId', m.customer_id,
  'checkoutStarted', m.checkout_started_at IS NOT NULL,
  'comped', t.billing_comped)), '[]')
FROM billing_provider_mappings m JOIN tenants t ON t.id = m.workspace_id"""

BUCKETS = (
    "not_bex",                # no bex tag at all — another product on the shared account
    "bound",                  # bex Customer with a default or attached payment method
    "tombstone",              # deleted workspace, retired on purpose
    "workspace_create",       # provisional workspace-create Customer
    "comped",                 # intended cardless owner
    "unbound_checkout",       # opened Checkout, never bound — reclaim target
    "unbound_no_checkout",    # minted without Checkout (pre-0123 checkout, emitter, admin)
    "unbound_unmapped",       # tagged but no local mapping (workspace gone or mode mismatch)
    "unbound_unknown",        # tagged, unbound, no mappings file supplied
)


def classify(customer: dict, has_attached_pm: bool, mappings: dict[str, dict] | None) -> str:
    """Pure bucket assignment for one Stripe Customer."""
    meta = customer.get("metadata") or {}
    workspace = meta.get(WORKSPACE_KEY, "")
    if not workspace and not meta.get(DELETED_KEY):
        # An unadopted Checkout-created Customer is untagged too; the reclaimer's
        # expired-session sweep owns those, keyed by the session, not by this.
        return "not_bex"
    default_pm = ((customer.get("invoice_settings") or {}).get("default_payment_method"))
    if default_pm or has_attached_pm:
        return "bound"
    if meta.get(DELETED_KEY):
        return "tombstone"
    if meta.get(ATTEMPT_KEY):
        return "workspace_create"
    if mappings is None:
        return "unbound_unknown"
    mapping = mappings.get(workspace)
    if mapping is None or mapping.get("customerId") != customer.get("id"):
        return "unbound_unmapped"
    if mapping.get("comped"):
        return "comped"
    if mapping.get("checkoutStarted"):
        return "unbound_checkout"
    return "unbound_no_checkout"


def list_customers() -> list[dict]:
    out: list[dict] = []
    after = None
    while True:
        args = ["get", "/v1/customers", "-d", "limit=100"]
        if after:
            args += ["-d", f"starting_after={after}"]
        page = stripe_json(args)
        out.extend(page.get("data", []))
        if not page.get("has_more"):
            return out
        after = out[-1]["id"]


def has_attached_pm(customer_id: str) -> bool:
    page = stripe_json(["get", "/v1/payment_methods", "-d", f"customer={customer_id}", "-d", "limit=1"])
    return bool(page.get("data"))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--mappings", type=Path, help="JSON array produced by MAPPINGS_SQL")
    parser.add_argument("--print-sql", action="store_true", help="print the read-only mappings query and exit")
    args = parser.parse_args()
    if args.print_sql:
        print(MAPPINGS_SQL)
        return 0
    mappings = None
    if args.mappings:
        rows = json.loads(args.mappings.read_text())
        mappings = {row["workspaceId"]: row for row in rows}
    counts: Counter[str] = Counter()
    members: dict[str, list[str]] = {name: [] for name in BUCKETS}
    for customer in list_customers():
        if customer.get("deleted"):
            continue
        meta = customer.get("metadata") or {}
        default_pm = (customer.get("invoice_settings") or {}).get("default_payment_method")
        bex = meta.get(WORKSPACE_KEY) or meta.get(DELETED_KEY)
        # Only a bex Customer without a default method needs the extra read.
        attached = has_attached_pm(customer["id"]) if bex and not default_pm else False
        bucket = classify(customer, attached, mappings)
        counts[bucket] += 1
        members[bucket].append(f"{customer['id']} {meta.get(WORKSPACE_KEY, '-')}")
    report = {
        "counts": {name: counts.get(name, 0) for name in BUCKETS},
        "total": sum(counts.values()),
        # Ids only for the buckets an operator acts on; bound/tombstone stay counts.
        "reclaimTargets": members["unbound_checkout"] + members["unbound_no_checkout"],
        "needsReview": members["unbound_unmapped"] + members["unbound_unknown"],
    }
    json.dump(report, sys.stdout, indent=2)
    print()
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except RuntimeError as exc:
        print(f"error: {exc}", file=sys.stderr)
        sys.exit(2)

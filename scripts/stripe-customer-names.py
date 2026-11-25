#!/usr/bin/env python3
# stripe-customer-names.py — one-off backfill (w6/m628 t004): rename each bex
# Stripe Customer still named after its workspace id (`tea-…`) to the
# workspace's name, so the Bill-to of its invoices prints the name, not the id.
#
# Until w6/m628 bex created every workspace's Customer with name = the
# workspace id. bex-api now names a new Customer after the workspace's label
# (its display name, else its slug: store.Tenant.Label, billing.EnsureCustomer)
# and follows a workspace rename (FollowWorkspaceRename); this script brings
# the Customers created before that up to date.
#
# The Stripe account is shared with other products, so only a Customer tagged
# with metadata bex_workspace=<tea-id> is ever considered. Of those, one is
# renamed only when
#   - its livemode matches this run's mode,
#   - it is not deleted and not retired (bex_deleted_at: a deleted workspace's
#     tombstone, kept for invoice retention),
#   - its name is still exactly its bex_workspace id and it carries no
#     bex_company_name. Any other name (a company name a member set in Billing
#     info, a name edited in the Customer Portal, or one already backfilled)
#     is the member's or already right, and is left alone,
#   - its workspace still exists in the control-plane `tenants` table, and
#   - that workspace's label is not the id itself (a signup workspace whose
#     slug is its id and which has no display name yet).
# Just before each write the Customer is read again and skipped if it no
# longer qualifies (a workspace rename or a portal edit since the listing).
#
# Dry run by default: prints counts and one line per planned rename with the
# Customer id and the workspace id only. Workspace names are other tenants'
# data and are printed only with --show-names.
#
# Usage:
#   scripts/stripe-customer-names.py                  # dry run, test mode
#   scripts/stripe-customer-names.py --live           # dry run, live mode
#   scripts/stripe-customer-names.py --live --apply   # rename, live mode
#   (add --show-names to print each planned name)
#
# Stripe: the `stripe` CLI with its own configured key, as
# stripe-billing-setup.py: TEST MODE unless --live.
# Control-plane DB (one read-only query), as funnel-counters.sh's run_sql:
#   BEX_CP_DB_URI        direct psql connection string; unset -> kubectl exec
#                        into the CNPG primary as postgres (export KUBECONFIG
#                        for the cluster that runs bex-system first)
#   BEX_SYSTEM_NAMESPACE (bex-system), BEX_CP_DB_CLUSTER (bex-db),
#   BEX_CP_DB_NAME (bex) CNPG exec-mode coordinates
#
# Exit: 0 done (or dry run); 1 at least one rename failed (the rest were
# applied, and a re-run is safe: a renamed Customer no longer matches);
# 2 the workspaces or the Customers could not be read (nothing was written).
import argparse
import json
import os
import subprocess
import sys
from collections import namedtuple

WORKSPACE_KEY = "bex_workspace"
DELETED_KEY = "bex_deleted_at"
# A company name a member set in Billing info (w6/m628); billing's
# namedByWorkspace treats such a Customer's name as the member's.
COMPANY_NAME_KEY = "bex_company_name"
STRIPE_GLOBAL_ARGS = []

# One JSON value, not psql rows: a workspace name may hold `|`, tabs or
# newlines, which unaligned output would split; JSON keeps them escaped. The
# label is store.Tenant.Label: the display name, else the slug.
LABELS_SQL = """SELECT COALESCE(json_agg(json_build_object(
  'id', id, 'label', COALESCE(NULLIF(display_name, ''), name)) ORDER BY id), '[]')
FROM tenants;
"""

# (key, report line), in report order. Every listed Customer lands in exactly
# one of not_bex, other_mode or a bucket under "seen".
COUNTS = (
    ("listed", "Customers listed"),
    ("not_bex", "  not bex (no bex_workspace, untouched)"),
    ("other_mode", "  bex, other mode (skipped)"),
    ("seen", "  bex Customers seen"),
    ("rename", "    to rename"),
    ("already_named", "    already named (left alone)"),
    ("retired", "    retired/deleted (skipped)"),
    ("workspace_gone", "    workspace gone (skipped)"),
    ("label_is_id", "    workspace label is its id (skipped)"),
)
SEEN = ("rename", "already_named", "retired", "workspace_gone", "label_is_id")

Rename = namedtuple("Rename", "customer_id workspace_id name")


def stripe(*args):
    """Run the stripe CLI, return its JSON. An error names only the method and
    path: the request data can carry a workspace name."""
    what = " ".join(args[:2])
    try:
        out = subprocess.run(["stripe", *args, *STRIPE_GLOBAL_ARGS],
                             capture_output=True, text=True, encoding="utf-8")
    except OSError as exc:
        raise RuntimeError(f"stripe {what}: {exc}") from exc
    text = out.stdout.strip()
    if out.returncode != 0 or not text.startswith(("{", "[")):
        # stripe CLI prints errors to stdout as text; surface them
        raise RuntimeError(f"stripe {what}\n{out.stdout}{out.stderr}")
    payload = json.loads(text)
    if isinstance(payload, dict) and payload.get("error"):
        error = payload["error"]
        message = error.get("message", "Stripe returned an error") if isinstance(error, dict) else str(error)
        raise RuntimeError(f"stripe {what}\n{message}")
    return payload


def run_sql(sql, env=None, run=subprocess.run):
    """SQL on stdin, unaligned tuples on stdout: a direct BEX_CP_DB_URI, else
    kubectl exec into the CNPG primary as the local postgres superuser."""
    env = os.environ if env is None else env
    if env.get("BEX_CP_DB_URI"):
        cmd = ["psql", env["BEX_CP_DB_URI"], "-v", "ON_ERROR_STOP=1", "-qAt"]
    else:
        namespace = env.get("BEX_SYSTEM_NAMESPACE") or "bex-system"
        cluster = env.get("BEX_CP_DB_CLUSTER") or "bex-db"
        database = env.get("BEX_CP_DB_NAME") or "bex"
        found = run(["kubectl", "-n", namespace, "get", "pod",
                     "-l", f"cnpg.io/cluster={cluster},cnpg.io/instanceRole=primary",
                     "-o", "jsonpath={.items[0].metadata.name}"],
                    capture_output=True, text=True, encoding="utf-8")
        primary = found.stdout.strip()
        if found.returncode != 0 or not primary:
            raise RuntimeError(f"no CNPG primary for cluster {cluster} in namespace {namespace} "
                               f"(set BEX_CP_DB_URI for a direct connection)\n{found.stderr}")
        cmd = ["kubectl", "-n", namespace, "exec", "-i", primary, "--",
               "psql", "-U", "postgres", "-d", database, "-v", "ON_ERROR_STOP=1", "-qAt"]
    out = run(cmd, input=sql, capture_output=True, text=True, encoding="utf-8")
    if out.returncode != 0:
        raise RuntimeError(f"control-plane query failed (exit {out.returncode})\n{out.stderr}")
    return out.stdout


def parse_labels(output):
    """LABELS_SQL's output -> {workspace id: label}. A parse error never echoes
    the output, which holds every workspace's name."""
    try:
        rows = json.loads(output)
        return {row["id"]: row["label"] or "" for row in rows}
    except (ValueError, TypeError, KeyError) as exc:
        raise RuntimeError(f"unexpected tenants query output ({type(exc).__name__})") from None


def read_labels():
    return parse_labels(run_sql(LABELS_SQL))


def customer_page_args(starting_after=None):
    args = ["get", "/v1/customers", "-d", "limit=100"]
    if starting_after:
        args += ["-d", f"starting_after={starting_after}"]
    return args


def list_customers(fetch_page):
    """Every Customer, following has_more with the last id as the cursor;
    fetch_page(starting_after) returns one list page."""
    customers = []
    starting_after = None
    while True:
        page = fetch_page(starting_after)
        data = page.get("data") or []
        customers.extend(data)
        if not page.get("has_more"):
            return customers
        if not data:
            raise RuntimeError("Stripe Customer list returned has_more with no data")
        starting_after = data[-1]["id"]


def classify(customer, labels, live):
    """The one COUNTS bucket a Customer falls in; only "rename" is written."""
    meta = customer.get("metadata") or {}
    workspace = meta.get(WORKSPACE_KEY) or ""
    if not workspace:
        return "not_bex"
    if customer.get("livemode") is not live:
        return "other_mode"
    if customer.get("deleted") or meta.get(DELETED_KEY):
        return "retired"
    if customer.get("name") != workspace or meta.get(COMPANY_NAME_KEY):
        return "already_named"
    if workspace not in labels:
        return "workspace_gone"
    label = labels[workspace]
    if not label.strip() or label == workspace:
        return "label_is_id"
    return "rename"


def plan(customers, labels, live):
    """-> ([Rename], counts) for the listed Customers and {workspace id: label}."""
    counts = {key: 0 for key, _ in COUNTS}
    renames = []
    for customer in customers:
        bucket = classify(customer, labels, live)
        counts["listed"] += 1
        counts[bucket] += 1
        if bucket in SEEN:
            counts["seen"] += 1
        if bucket == "rename":
            workspace = customer["metadata"][WORKSPACE_KEY]
            renames.append(Rename(customer["id"], workspace, labels[workspace]))
    return renames, counts


def describe(rename, show_names):
    line = f"{rename.customer_id} {rename.workspace_id}"
    if show_names:
        line += f" -> {json.dumps(rename.name, ensure_ascii=False)}"
    return line


def apply_renames(renames, labels, live, stripe_fn, show_names):
    """Write each planned name; -> (renamed, skipped, failed)."""
    renamed = skipped = failed = 0
    for rename in renames:
        try:
            current = stripe_fn("get", f"/v1/customers/{rename.customer_id}")
            if (classify(current, labels, live) != "rename"
                    or current["metadata"][WORKSPACE_KEY] != rename.workspace_id):
                skipped += 1
                print(f"  skipped {describe(rename, False)}: changed since it was listed")
                continue
            updated = stripe_fn("post", f"/v1/customers/{rename.customer_id}", "-d", f"name={rename.name}")
            if updated.get("name") != rename.name:
                raise RuntimeError("Stripe did not return the new name")
        except RuntimeError as exc:
            failed += 1
            message = str(exc) if show_names else str(exc).replace(rename.name, "<name>")
            print(f"  FAILED {describe(rename, False)}: {message}", file=sys.stderr)
            continue
        renamed += 1
        print(f"  renamed {describe(rename, show_names)}")
    return renamed, skipped, failed


def parse_args(argv):
    parser = argparse.ArgumentParser(
        description="Rename bex Stripe Customers still named after their workspace id "
                    "to the workspace's name (dry run, test mode by default).")
    parser.add_argument("--live", action="store_true",
                        help="use live-mode Stripe objects (default: test mode)")
    parser.add_argument("--apply", action="store_true",
                        help="perform the renames (default: dry run, writes nothing)")
    parser.add_argument("--show-names", action="store_true",
                        help="print each planned workspace name (other tenants' data; off by default)")
    return parser.parse_args(argv)


def main(argv=None, stripe_fn=None, read_labels_fn=None):
    args = parse_args(argv)
    global STRIPE_GLOBAL_ARGS
    STRIPE_GLOBAL_ARGS = ["--live"] if args.live else []
    stripe_fn = stripe_fn or stripe
    read_labels_fn = read_labels_fn or read_labels
    try:
        labels = read_labels_fn()
        customers = list_customers(lambda after: stripe_fn(*customer_page_args(after)))
    except (RuntimeError, OSError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 2
    renames, counts = plan(customers, labels, args.live)

    mode = "live" if args.live else "test"
    print(f"== bex Stripe Customer names: {mode} mode, {'apply' if args.apply else 'dry run'} ==")
    for key, title in COUNTS:
        print(f"{title + ':':<44} {counts[key]}")
    if not args.apply:
        if renames:
            print("\nplanned renames (Customer workspace):")
            for rename in renames:
                print(f"  {describe(rename, args.show_names)}")
        print("\nDry run: nothing written. Re-run with --apply to rename.")
        return 0

    print()
    renamed, skipped, failed = apply_renames(renames, labels, args.live, stripe_fn, args.show_names)
    print(f"\nrenamed {renamed}, skipped {skipped} (changed since listed), failed {failed}")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# stripe-billing-setup.py — create/refresh the bex Stripe Billing config
# (meters + products + prices) from lego/backend/internal/pricing/pricing.yaml,
# so pricing.yaml stays the single source of truth (m50, docs/ADR040-billing-metronome.md).
#
# Idempotent: skips a meter whose event_name already exists (reactivates a
# deactivated one), skips a price whose lookup_key already exists, and
# deactivates the superseded kind-level meters. Uses the `stripe` CLI's
# configured key — TEST MODE unless the CLI is paired to a live account.
#
# Model (why these event_names / units):
#   instance_seconds → per-(resource_kind, tier) meter  event=instance_seconds.<rk>.<tier>
#                      value = seconds, price = usdPerSecond×100 cents/sec (paid tiers only)
#   egress_bytes     → re-based to per-GiB               event=egress_gib
#                      value = bytes/2^30, price = 1.5 cents/GiB  (Stripe's 12-decimal
#                      unit_amount_decimal can't express $1.4e-11/byte)
#   build_seconds    → per-second                        event=build_seconds
#   storage_gb_seconds → re-based to per-GB-hour         event=storage_gb_hours
#                      value = gb_seconds/3600, price = usdPerGBSecond×3600×100
#
# The billing client (internal/billing/stripe.go) composes the identical
# event_names + re-bases the same way — the two must agree.
import json
import subprocess
import sys
from decimal import Decimal
from pathlib import Path

PRICING = Path(__file__).resolve().parents[1] / "lego/backend/internal/pricing/pricing.yaml"
BYTES_PER_GIB = Decimal(1073741824)
SECONDS_PER_HOUR = Decimal(3600)
# Kind-level meters from the shadow phase that per-tier / re-based meters replace.
SUPERSEDED = ["instance_seconds", "egress_bytes", "storage_gb_seconds"]


def dec(s):
    return Decimal(str(s).split("#")[0].strip())


def q12(d):
    """Quantize to Stripe's max 12-decimal unit_amount_decimal precision."""
    return Decimal(d).quantize(Decimal("1e-12"))


def plain(d):
    """Decimal → plain decimal string (no exponent, no trailing zeros)."""
    d = Decimal(d).normalize()
    s = format(d, "f")
    return s


def parse_pricing():
    """Minimal parser for pricing.yaml → list of (event_name, display, cents_per_unit)."""
    section = None
    tier = None
    dims = []
    for raw in PRICING.read_text().splitlines():
        line = raw.split("#", 1)[0].rstrip()
        if not line.strip():
            continue
        if not line.startswith(" ") and line.endswith(":"):
            section = line[:-1].strip()
            tier = None
            continue
        stripped = line.strip()
        if stripped.startswith("- tier:"):
            tier = stripped.split(":", 1)[1].strip()
        elif ":" in stripped:
            key, val = (x.strip() for x in stripped.split(":", 1))
            if key == "tier":
                tier = val
                continue
            if key not in ("usdPerSecond", "usdPerByte", "usdPerGBSecond"):
                continue  # skip version:, etc.
            rate = dec(val)
            rk = {"compute": "service", "postgres": "postgres", "keyvalue": "key_value"}.get(section)
            if rk is not None:  # instance_seconds, per tier
                if rate > 0 and tier and tier != "free":
                    dims.append((f"instance_seconds.{rk}.{tier}",
                                 f"bex instance_seconds {rk} {tier}",
                                 q12(rate * 100)))
            elif section == "bandwidth":
                dims.append(("egress_gib", "bex egress (per GiB)", q12(rate * BYTES_PER_GIB * 100)))
            elif section == "build":
                dims.append(("build_seconds", "bex build_seconds", q12(rate * 100)))
            elif section == "storage":
                dims.append(("storage_gb_hours", "bex storage (per GB-hour)", q12(rate * SECONDS_PER_HOUR * 100)))
    return dims


def stripe(*args):
    out = subprocess.run(["stripe", *args], capture_output=True, text=True)
    if out.returncode != 0 or (out.stdout.strip().startswith("{") is False and out.stdout.strip().startswith("[") is False):
        # stripe CLI prints errors to stdout as text; surface them
        raise RuntimeError(f"stripe {' '.join(args)}\n{out.stdout}{out.stderr}")
    return json.loads(out.stdout)


def existing_meters():
    m = {}
    data = stripe("billing", "meters", "list", "--limit", "100")["data"]
    for it in data:
        m[it["event_name"]] = it
    return m


def ensure_meter(event_name, display, meters):
    if event_name in meters:
        it = meters[event_name]
        if it["status"] != "active":
            stripe("billing", "meters", "reactivate", it["id"])
        return it["id"], "exists"
    it = stripe("billing", "meters", "create",
                "-d", f"display_name={display}",
                "-d", f"event_name={event_name}",
                "-d", "default_aggregation[formula]=sum",
                "-d", "value_settings[event_payload_key]=value",
                "-d", "customer_mapping[type]=by_id",
                "-d", "customer_mapping[event_payload_key]=stripe_customer_id")
    meters[event_name] = it
    return it["id"], "created"


def price_exists(lookup_key):
    data = stripe("prices", "list", "-d", f"lookup_keys[0]={lookup_key}", "--limit", "1")["data"]
    return len(data) > 0


def ensure_price(event_name, display, cents, meter_id):
    if price_exists(event_name):
        return "exists"
    stripe("prices", "create",
           "-d", "currency=usd",
           "-d", f"unit_amount_decimal={plain(cents)}",
           "-d", "recurring[interval]=month",
           "-d", "recurring[usage_type]=metered",
           "-d", f"recurring[meter]={meter_id}",
           "-d", f"lookup_key={event_name}",
           "-d", f"product_data[name]={display}")
    return "created"


def main():
    dims = parse_pricing()
    meters = existing_meters()
    print(f"== bex Stripe Billing setup — {len(dims)} priced dimensions from pricing.yaml ==\n")
    print(f"{'event_name':<38} {'unit_amount_decimal (¢)':<24} meter        price")
    for event_name, display, cents in dims:
        mid, mstat = ensure_meter(event_name, display, meters)
        pstat = ensure_price(event_name, display, cents, mid)
        print(f"{event_name:<38} {plain(cents):<24} {mstat:<12} {pstat}")

    print("\n== deactivating superseded kind-level meters ==")
    for name in SUPERSEDED:
        it = meters.get(name)
        if it and it["status"] == "active":
            stripe("billing", "meters", "deactivate", it["id"])
            print(f"  deactivated {name} ({it['id']})")
        else:
            print(f"  {name}: already gone/inactive")
    print("\nDone.")


if __name__ == "__main__":
    try:
        main()
    except RuntimeError as e:
        print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)

#!/usr/bin/env python3
# stripe-billing-setup.py — create/refresh the bex Stripe Billing config
# (meters + products + prices + the Mode-B comp coupon) from
# lego/backend/internal/pricing/pricing.yaml, so pricing.yaml stays the single
# source of truth (m50, docs/ADR040-billing-metronome.md).
#
# Idempotent: skips a meter whose event_name already exists (reactivates a
# deactivated one), validates and skips a price whose lookup_key already
# exists, validates/creates the stable comp coupon, and deactivates the
# superseded kind-level meters. Uses the `stripe` CLI's configured key — TEST
# MODE unless --live is passed explicitly to the CLI configuration/run.
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
#   sandbox_compute_seconds → milli-vCPU-equivalent sec  event=sandbox_compute_seconds
#                      value = weighted seconds, price = usdPerWeightedSecond×100
#
# The billing client (internal/billing/stripe.go) composes the identical
# event_names + re-bases the same way — the two must agree.
import argparse
import json
import subprocess
import sys
from decimal import Decimal
from pathlib import Path
from urllib.parse import urlsplit

PRICING = Path(__file__).resolve().parents[1] / "lego/backend/internal/pricing/pricing.yaml"
BYTES_PER_GIB = Decimal(1073741824)
SECONDS_PER_HOUR = Decimal(3600)
# Kind-level meters from the shadow phase that per-tier / re-based meters replace.
SUPERSEDED = ["instance_seconds", "egress_bytes", "storage_gb_seconds"]
COMP_COUPON_ID = "bex-comp-100"
STRIPE_GLOBAL_ARGS = []
PORTAL_METADATA_KEY = "bex_billing_portal"


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
            if key not in ("usdPerSecond", "usdPerByte", "usdPerGBSecond", "usdPerWeightedSecond"):
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
            elif section == "sandbox":
                dims.append(("sandbox_compute_seconds", "bex sandbox compute (weighted second)", q12(rate * 100)))
    return dims


def stripe(*args):
    out = subprocess.run(["stripe", *args, *STRIPE_GLOBAL_ARGS], capture_output=True, text=True)
    if out.returncode != 0 or (out.stdout.strip().startswith("{") is False and out.stdout.strip().startswith("[") is False):
        # stripe CLI prints errors to stdout as text; surface them
        raise RuntimeError(f"stripe {' '.join(args)}\n{out.stdout}{out.stderr}")
    payload = json.loads(out.stdout)
    if isinstance(payload, dict) and payload.get("error"):
        error = payload["error"]
        message = error.get("message", "Stripe returned an error") if isinstance(error, dict) else str(error)
        raise RuntimeError(f"stripe {' '.join(args)}\n{message}")
    return payload


def list_all(*resource):
    """List every object for a Stripe CLI resource, following id cursors."""
    items = []
    starting_after = None
    while True:
        args = [*resource, "list", "--limit", "100"]
        if starting_after:
            args.extend(["-d", f"starting_after={starting_after}"])
        page = stripe(*args)
        data = page["data"]
        items.extend(data)
        if not page.get("has_more"):
            return items
        if not data:
            raise RuntimeError(f"Stripe {' '.join(resource)} list returned has_more with no data")
        starting_after = data[-1]["id"]


def existing_meters():
    m = {}
    for it in list_all("billing", "meters"):
        m[it["event_name"]] = it
    return m


def ensure_meter(event_name, display, meters):
    if event_name in meters:
        it = meters[event_name]
        aggregation = it.get("default_aggregation", {}).get("formula")
        customer_mapping = it.get("customer_mapping", {})
        value_settings = it.get("value_settings", {})
        if (aggregation not in (None, "sum")
                or customer_mapping.get("type") not in (None, "by_id")
                or customer_mapping.get("event_payload_key") not in (None, "stripe_customer_id")
                or value_settings.get("event_payload_key") not in (None, "value")):
            raise RuntimeError(
                f"meter {event_name} exists with incompatible aggregation/mapping; "
                "repair it in Stripe before enabling billing")
        if it["status"] != "active":
            stripe("billing", "meters", "reactivate", it["id"])
            it["status"] = "active"
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


def existing_price(lookup_key):
    data = stripe("prices", "list",
                  "-d", f"lookup_keys[0]={lookup_key}",
                  "-d", "active=true", "--limit", "2")["data"]
    if len(data) > 1:
        raise RuntimeError(f"multiple active Stripe prices use lookup_key {lookup_key}")
    return data[0] if data else None


def ensure_product_tax(product_id, tax_code):
    product = stripe("products", "retrieve", product_id)
    current = product.get("tax_code")
    current_id = current.get("id") if isinstance(current, dict) else current
    if current_id != tax_code:
        stripe("products", "update", product_id, "-d", f"tax_code={tax_code}")


def ensure_price(event_name, display, cents, meter_id, tax_code=None, tax_behavior=None):
    existing = existing_price(event_name)
    if existing:
        recurring = existing.get("recurring") or {}
        actual_cents = Decimal(str(existing.get("unit_amount_decimal")))
        if (existing.get("currency") != "usd"
                or existing.get("type") != "recurring"
                or recurring.get("interval") != "month"
                or recurring.get("usage_type") != "metered"
                or recurring.get("meter") != meter_id
                or actual_cents != cents):
            raise RuntimeError(
                f"active price {existing.get('id')} for {event_name} does not match "
                f"pricing.yaml/meter {meter_id}; deactivate or repair it before enabling billing")
        if tax_behavior:
            current_behavior = existing.get("tax_behavior", "unspecified")
            if current_behavior == "unspecified":
                stripe("prices", "update", existing["id"],
                       "-d", f"tax_behavior={tax_behavior}")
            elif current_behavior != tax_behavior:
                raise RuntimeError(
                    f"active price {existing.get('id')} for {event_name} has "
                    f"tax_behavior={current_behavior}, want {tax_behavior}; tax behavior "
                    "cannot be changed after it is fixed")
            product = existing.get("product")
            product_id = product.get("id") if isinstance(product, dict) else product
            if not product_id:
                raise RuntimeError(f"price {existing.get('id')} has no Product")
            ensure_product_tax(product_id, tax_code)
        return "exists"
    args = [
        "prices", "create",
        "-d", "currency=usd",
        "-d", f"unit_amount_decimal={plain(cents)}",
        "-d", "recurring[interval]=month",
        "-d", "recurring[usage_type]=metered",
        "-d", f"recurring[meter]={meter_id}",
        "-d", f"lookup_key={event_name}",
        "-d", f"product_data[name]={display}",
    ]
    if tax_behavior:
        args.extend(["-d", f"tax_behavior={tax_behavior}",
                     "-d", f"product_data[tax_code]={tax_code}"])
    stripe(*args)
    return "created"


def existing_coupons():
    return {it["id"]: it for it in list_all("coupons")}


def ensure_comp_coupon(coupons):
    existing = coupons.get(COMP_COUPON_ID)
    if existing:
        percent_off = existing.get("percent_off")
        if (not existing.get("valid", True)
                or percent_off is None
                or Decimal(str(percent_off)) != Decimal(100)
                or existing.get("duration") != "forever"):
            raise RuntimeError(
                f"coupon {COMP_COUPON_ID} exists but is not a valid perpetual 100%-off coupon")
        return "exists"
    coupon = stripe("coupons", "create",
                    "-d", f"id={COMP_COUPON_ID}",
                    "-d", "percent_off=100",
                    "-d", "duration=forever",
                    "-d", "name=bex comp — 100% off")
    coupons[COMP_COUPON_ID] = coupon
    return "created"


def validate_tax_gate(tax_code, tax_behavior, live):
    if bool(tax_code) != bool(tax_behavior):
        raise RuntimeError("--tax-code and --tax-behavior must be supplied together")
    if not tax_code:
        return 0
    if not tax_code.startswith("txcd_"):
        raise RuntimeError("--tax-code must be an operator-confirmed canonical Stripe txcd_* value")
    registrations = stripe("tax", "registrations", "list",
                           "-d", "status=active", "--limit", "100")["data"]
    matching_mode = [r for r in registrations if bool(r.get("livemode")) == live]
    if not matching_mode:
        mode = "live" if live else "test"
        raise RuntimeError(
            f"no active {mode}-mode Stripe Tax registration; refusing to mark the catalog tax-ready")
    return len(matching_mode)


def ensure_portal_configuration(dashboard_url):
    if not dashboard_url:
        return None, "skipped"
    parsed = urlsplit(dashboard_url)
    if (parsed.scheme != "https" or not parsed.hostname or parsed.username
            or parsed.password or parsed.path not in ("", "/")
            or parsed.query or parsed.fragment):
        raise RuntimeError("--dashboard-url must be an absolute HTTPS origin")
    origin = f"{parsed.scheme}://{parsed.netloc}"
    return_url = origin + "/usage"
    configs = list_all("billing_portal", "configurations")
    tagged = [c for c in configs if (c.get("metadata") or {}).get(PORTAL_METADATA_KEY) == "true"]
    if len(tagged) > 1:
        raise RuntimeError(f"multiple portal configurations carry {PORTAL_METADATA_KEY}=true")
    common = [
        "-d", f"default_return_url={return_url}",
        "-d", "features[payment_method_update][enabled]=true",
        "-d", "features[invoice_history][enabled]=true",
        "-d", "features[customer_update][enabled]=true",
        "-d", "features[customer_update][allowed_updates][0]=address",
        "-d", "features[customer_update][allowed_updates][1]=email",
        "-d", "features[customer_update][allowed_updates][2]=name",
        "-d", "features[customer_update][allowed_updates][3]=tax_id",
        "-d", "features[subscription_cancel][enabled]=false",
        "-d", "features[subscription_update][enabled]=false",
        "-d", f"metadata[{PORTAL_METADATA_KEY}]=true",
    ]
    if tagged:
        config = stripe("billing_portal", "configurations", "update", tagged[0]["id"], *common)
        return config["id"], "exists"
    config = stripe("billing_portal", "configurations", "create",
                    "-d", "name=bex billing portal", *common)
    return config["id"], "created"


def main(live=False, tax_code=None, tax_behavior=None, dashboard_url=None):
    global STRIPE_GLOBAL_ARGS
    STRIPE_GLOBAL_ARGS = ["--live"] if live else []
    dims = parse_pricing()
    meters = existing_meters()
    coupons = existing_coupons()
    registration_count = validate_tax_gate(tax_code, tax_behavior, live)
    print(f"== bex Stripe Billing setup — {len(dims)} priced dimensions from pricing.yaml ==\n")
    print(f"{'event_name':<38} {'unit_amount_decimal (¢)':<24} meter        price")
    for event_name, display, cents in dims:
        mid, mstat = ensure_meter(event_name, display, meters)
        pstat = ensure_price(event_name, display, cents, mid, tax_code, tax_behavior)
        print(f"{event_name:<38} {plain(cents):<24} {mstat:<12} {pstat}")

    print("\n== deactivating superseded kind-level meters ==")
    for name in SUPERSEDED:
        it = meters.get(name)
        if it and it["status"] == "active":
            stripe("billing", "meters", "deactivate", it["id"])
            print(f"  deactivated {name} ({it['id']})")
        else:
            print(f"  {name}: already gone/inactive")

    print("\n== Mode-B comp coupon ==")
    print(f"  {COMP_COUPON_ID}: {ensure_comp_coupon(coupons)}")

    print("\n== Customer Portal ==")
    portal_id, portal_status = ensure_portal_configuration(dashboard_url)
    if portal_id:
        print(f"  configuration: {portal_status} ({portal_id})")
        print(f"  set BEX_STRIPE_PORTAL_CONFIGURATION_ID={portal_id} in the runtime Secret")
    else:
        print("  skipped (pass --dashboard-url to provision the scoped portal)")

    print("\n== Stripe Tax gate ==")
    if tax_code:
        print(f"  ready: code={tax_code} behavior={tax_behavior} active_registrations={registration_count}")
        print("  set BEX_STRIPE_TAX_CODE and BEX_STRIPE_TAX_BEHAVIOR in the runtime Secret")
    else:
        print("  unconfigured (safe): no tax code was guessed and automatic tax stays off")
    print("\nDone.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Provision the bex Stripe Billing catalog (test mode by default)."
    )
    parser.add_argument(
        "--live",
        action="store_true",
        help="modify live-mode Stripe objects (default: test mode)",
    )
    parser.add_argument(
        "--tax-code",
        help="operator-confirmed canonical Stripe product tax code (txcd_*); requires --tax-behavior and an active registration",
    )
    parser.add_argument(
        "--tax-behavior",
        choices=("exclusive", "inclusive"),
        help="whether catalog prices exclude or include tax; requires --tax-code",
    )
    parser.add_argument(
        "--dashboard-url",
        help="trusted HTTPS dashboard origin; provisions a scoped Customer Portal configuration",
    )
    args = parser.parse_args()
    try:
        main(live=args.live, tax_code=args.tax_code, tax_behavior=args.tax_behavior,
             dashboard_url=args.dashboard_url)
    except RuntimeError as e:
        print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)

#!/usr/bin/env python3
"""Secret-safe Stripe test-mode usage reconciliation for m53."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import subprocess
import sys
from collections import Counter, defaultdict
from decimal import Decimal, InvalidOperation
from pathlib import Path


BYTES_PER_GIB = Decimal(1073741824)
SECONDS_PER_HOUR = Decimal(3600)


def decimal(value: object) -> Decimal:
    try:
        return Decimal(str(value))
    except InvalidOperation as exc:
        raise ValueError(f"invalid decimal quantity {value!r}") from exc


def normalized_value(row: dict) -> Decimal:
    value = decimal(row["quantity"])
    if row["kind"] == "egress_bytes":
        return value / BYTES_PER_GIB
    if row["kind"] == "storage_gb_seconds":
        return value / SECONDS_PER_HOUR
    return value


def compare(local: dict, summaries: dict[str, object], invoice_lines: list[dict]) -> dict:
    """Compare local rows with provider aggregates; never mutate either side."""
    expected: dict[str, Decimal] = defaultdict(Decimal)
    states = Counter()
    transaction_ids = Counter()
    problems: list[dict[str, str]] = []

    for row in local.get("rows", []):
        state = row.get("state", "")
        states[state] += 1
        transaction_id = row.get("transactionId", "")
        if transaction_id:
            transaction_ids[transaction_id] += 1
        if state in {"rejected", "ambiguous"}:
            problems.append(
                {
                    "type": state,
                    "transactionId": transaction_id,
                    "eventName": row.get("eventName", ""),
                    "errorCode": row.get("errorCode", ""),
                }
            )
            continue
        event_name = row.get("eventName", "")
        if state == "emitted" and event_name and event_name != "local_zero_rate":
            expected[event_name] += normalized_value(row)

    for transaction_id, count in transaction_ids.items():
        if count > 1:
            problems.append(
                {
                    "type": "duplicate_local_transaction",
                    "transactionId": transaction_id,
                    "eventName": "",
                    "errorCode": "",
                }
            )

    dimensions = []
    for event_name in sorted(set(expected) | set(summaries)):
        provider = decimal(summaries.get(event_name, 0))
        local_value = expected.get(event_name, Decimal(0))
        matched = provider == local_value
        dimensions.append(
            {
                "eventName": event_name,
                "local": format(local_value, "f"),
                "stripe": format(provider, "f"),
                "match": matched,
            }
        )
        if not matched:
            problems.append(
                {
                    "type": "quantity_mismatch",
                    "transactionId": "",
                    "eventName": event_name,
                    "errorCode": "",
                }
            )

    line_counts = Counter(line.get("lookupKey", "") for line in invoice_lines if line.get("lookupKey"))
    duplicate_lines = sorted(key for key, count in line_counts.items() if count > 1)
    for key in duplicate_lines:
        problems.append(
            {
                "type": "duplicate_invoice_line",
                "transactionId": "",
                "eventName": key,
                "errorCode": "",
            }
        )

    invoice_by_key = {line.get("lookupKey", ""): line for line in invoice_lines if line.get("lookupKey")}
    for event_name, local_value in sorted(expected.items()):
        line = invoice_by_key.get(event_name)
        if line is None:
            problems.append(
                {
                    "type": "missing_invoice_line",
                    "transactionId": "",
                    "eventName": event_name,
                    "errorCode": "",
                }
            )
            continue
        quantity = line.get("quantity")
        if quantity is None or decimal(quantity) != local_value:
            problems.append(
                {
                    "type": "invoice_quantity_mismatch",
                    "transactionId": "",
                    "eventName": event_name,
                    "errorCode": "",
                }
            )
        if line.get("amount") is None or not line.get("currency"):
            problems.append(
                {
                    "type": "invoice_amount_missing",
                    "transactionId": "",
                    "eventName": event_name,
                    "errorCode": "",
                }
            )

    return {
        "workspaceId": local.get("workspaceId", ""),
        "livemode": local.get("livemode"),
        "states": dict(sorted(states.items())),
        "dimensions": dimensions,
        "invoiceLines": invoice_lines,
        "problems": problems,
        "ok": not problems and all(item["match"] for item in dimensions),
    }


def stripe_json(args: list[str]) -> dict:
    env = os.environ.copy()
    key = env.get("BEX_STRIPE_SECRET_KEY", "")
    if not key.startswith(("rk_test_", "sk_test_")):
        raise RuntimeError("BEX_STRIPE_SECRET_KEY must be a Stripe test-mode key; live mode is refused")
    env["STRIPE_API_KEY"] = key
    completed = subprocess.run(
        ["stripe", *args, "--color", "off"],
        check=False,
        capture_output=True,
        text=True,
        env=env,
    )
    try:
        body = json.loads(completed.stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"Stripe CLI returned non-JSON output (exit {completed.returncode})") from exc
    if completed.returncode != 0 or body.get("error"):
        error = body.get("error", {})
        code = error.get("code") or error.get("type") or "provider_error"
        raise RuntimeError(f"Stripe test-mode API request failed: {code}")
    if body.get("livemode") is True or any(item.get("livemode") is True for item in body.get("data", [])):
        raise RuntimeError("Stripe response contained a live-mode object; refusing")
    return body


def unix_seconds(value: str) -> int:
    parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        raise ValueError("timestamp must include a timezone")
    return int(parsed.timestamp())


def provider_evidence(local: dict, start: str, end: str) -> tuple[dict[str, Decimal], list[dict]]:
    if local.get("livemode") is True:
        raise RuntimeError("local provider mapping is live mode; refusing")
    customer = local.get("customerId", "")
    subscription = local.get("subscriptionId", "")
    names = sorted(
        {
            row.get("eventName", "")
            for row in local.get("rows", [])
            if row.get("state") == "emitted" and row.get("eventName") not in {"", "local_zero_rate"}
        }
    )
    if names and not customer:
        raise RuntimeError("emitted rows have no Stripe Customer mapping")
    if names and not subscription:
        raise RuntimeError("emitted rows have no Stripe Subscription mapping")

    meters = stripe_json(["get", "/v1/billing/meters", "--limit", "100"])
    by_name = {item.get("event_name"): item for item in meters.get("data", [])}
    summaries: dict[str, Decimal] = {}
    for name in names:
        meter = by_name.get(name)
        if not meter:
            raise RuntimeError(f"Stripe meter is missing for event {name}")
        response = stripe_json(
            [
                "get",
                f"/v1/billing/meters/{meter['id']}/event_summaries",
                "-d",
                f"customer={customer}",
                "-d",
                f"start_time={unix_seconds(start)}",
                "-d",
                f"end_time={unix_seconds(end)}",
                "-d",
                "value_grouping_window=hour",
                "--limit",
                "100",
            ]
        )
        summaries[name] = sum(
            (decimal(item.get("aggregated_value", 0)) for item in response.get("data", [])),
            Decimal(0),
        )

    invoice_lines: list[dict] = []
    if subscription:
        preview = stripe_json(
            [
                "post",
                "/v1/invoices/create_preview",
                "-d",
                f"customer={customer}",
                "-d",
                f"subscription={subscription}",
                "-e",
                "lines.data.pricing.price_details.price",
            ]
        )
        for line in (preview.get("lines") or {}).get("data", []):
            pricing = ((line.get("pricing") or {}).get("price_details") or {}).get("price") or {}
            invoice_lines.append(
                {
                    "id": line.get("id", ""),
                    "lookupKey": pricing.get("lookup_key", ""),
                    "quantity": line.get("quantity"),
                    "amount": line.get("amount"),
                    "currency": line.get("currency", ""),
                }
            )
    return summaries, invoice_lines


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--local", type=Path, required=True)
    parser.add_argument("--start", required=True)
    parser.add_argument("--end", required=True)
    parser.add_argument("--provider-fixture", type=Path)
    args = parser.parse_args()
    local = json.loads(args.local.read_text())
    if args.provider_fixture:
        fixture = json.loads(args.provider_fixture.read_text())
        summaries = fixture.get("summaries", {})
        lines = fixture.get("invoiceLines", [])
    else:
        summaries, lines = provider_evidence(local, args.start, args.end)
    result = compare(local, summaries, lines)
    print(json.dumps(result, sort_keys=True, indent=2))
    return 0 if result["ok"] else 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, RuntimeError, ValueError) as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        raise SystemExit(2)

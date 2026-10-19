# CLI sweep 518 — service type/plan gate dialects

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build `218dffcd1`, 2026-10-19T09:22Z. Refusal-only sweep: the workspace service count was 8 before and after — nothing was created by any probe.

| probe | observed result |
| --- | --- |
| `private_service --plan free` | Exit `1`, server `400`: "a private_service requires a paid plan; plan must be one of starter\|standard\|pro\|pro_plus\|pro_max\|pro_ultra" — Render's rule (private services have no free tier), with the valid set named. |
| `background_worker --plan free`, `cron_job --plan free` | Same shaped refusal per type. The paid-only messages correctly **omit** `free` from the valid set — live confirmation that the w4/352 fix ("a mistyped plan on a paid-only type is never told free is valid") is deployed and holds through the CLI. |
| `--type bogus_type` | Exit `1` client-side with the closed enum named; no request sent. |
| `web_service --plan bogusplan` | Exit `1`, server `400`: "plan must be one of free\|starter\|…" — for web the list correctly includes `free`. |

## Safety and unrun checks

- Paid plan names were never submitted with an intent to create (a successful paid create would be a purchase); the paid-only types therefore have refusal coverage only, per hunt rules.
- Unrun: `static_site --plan <anything>` beyond sweep 499's planless-static guard, spec-based plan names (`0.5c-512mb` class) against bex's catalog — recorded in the compat ledger as a partially-closed intended divergence (w8/011 aliases), not re-probed here.
- Deploy lag means deployed revisions can precede local `main`; the w4/352 wording matching its fix exactly suggests production is at or past it for this path.

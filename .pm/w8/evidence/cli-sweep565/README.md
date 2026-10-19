# CLI sweep 565 — `ipAllowList` CIDR dialects, both spellings (owned fixture)

**Verdict: no bug filed.** Same isolated QA session (`puncsky@gmail.com`), `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T18:44–18:47Z. Owned fixture `qa565-allowlist` (`srv-dbb69r7o17is73furrc0`, free hello-app) via same-credential REST (`/tmp/qa501/allow565.py`) plus blueprint validate; deleted with REST + URL 404 proof, workspace back to 8. Enforcement flips themselves were covered in sweep 458; this sweep targets the validation dialects around `core.ValidateAllowList`/`netip.ParsePrefix` (the shared write/enforcement grammar).

- **Refusals (REST PATCH `serviceDetails.ipAllowList`, state verified unmoved after the batch):** every malformed CIDR echoes the offending value — `not-a-cidr`, bare host `1.2.3.4`, `1.2.3.4/33`, `::1/129`, zero-padded `10.0.0.0/024` (the netip-vs-`net.ParseCIDR` divergence the code comments call out), empty string, missing `cidrBlock` (validates as `""`), and a bad second entry (no partial write). Shape errors are layer-named: wrong-typed `cidrBlock`, a bare-string entry ("invalid JSON request body"), the blueprint's `source` key ("unknown field"), and 1001 entries ("ipAllowList has 1001 entries; the limit is 1000").
- **Acceptances round-trip verbatim:** host-bits-set `203.0.113.5/24` (stored as written, not masked — prefix containment is the enforcement semantic), IPv6 `2001:db8::/32`, duplicate entries, a 2000-character description (metadata, uncapped). An unset/cleared list reads as an **absent** key on service reads — correct, since `ipAllowList` is not in `webServiceDetails.required` in the pinned schema (the `[]`-not-null rule of w6/m109 binds the datastore reads where it is required).
- **Enforcement sanity (one flip):** restricting to documentation-range `203.0.113.0/24` made the platform URL answer `403 Forbidden` within 10 s; clearing restored `200` just as fast.
- **Blueprint dialect (validate-only):** the `source:` spelling validates; `cidrBlock:` in a blueprint is refused twice over (missing `source` + additional property) — mirror image of REST refusing `source`, each surface's error teaching its own spelling; bad CIDR hits the same semantic message; bare-string entries fail the schema type check.

One transient `ConnectionResetError` hit the probe mid-run (harness network, immediately retryable) — not a product signal.

## Unrun

- Datastore (Postgres/Key Value) allowlist enforcement through the SNI proxies — shared validator, different data plane; free slots occupied by baselines regardless.
- Entry-count boundary at exactly 1000 accepted entries (1001 refusal proven; the accept side is source-covered).

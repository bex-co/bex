# w4 · m29 — Retire the ipAllowList legacy-string shim (one-shot CR normalization + schema tightening)

**Worker:** worker4 **Goal:** `Database`/`KeyValue` `spec.ipAllowList` regains apiserver-side validation: an idempotent one-shot normalization rewrites legacy bare-CIDR-string entries to `{cidr}` objects, the CRD field returns to a structural object schema (required `cidr`), and the `IPAllowEntry.UnmarshalJSON` union decoder is demoted to a test fixture. **Status:** done 2026-07-15 — normalizer shipped + prod fleet verified clean (phase 1, `fbfee9a7`), then a separate deploy restored the structural CRD schema (required `cidr`, admission-rejects strings), deleted the union decoder (test fixture only), inverted the envtest, and swept the falsified docs

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Idempotent one-shot normalization: bare strings → `{cidr}` (RC5 backfill precedent) — **DONE** | 45m | —          |
| t002 | Verify + record zero string-shaped CRs remain (prod + local evidence) — **DONE** | 20m | t001       |
| t003 | Restore structural object schema on the CRD; drop Schemaless/PreserveUnknown — **DONE** | 30m | t002       |
| t004 | Demote the `UnmarshalJSON` union decoder to a test fixture — **DONE** | 20m | t003       |
| t005 | Simplify — **DONE** | 20m | t004       |
| t006 | Test coverage — **DONE** | 30m | t004       |
| t007 | Closeout — **DONE** | 15m | t006       |

## Definition of done

No string-shaped `ipAllowList` entry exists in any `Database`/`KeyValue` CR (evidence recorded from prod and the local dev environments); the CRD field is structural again — `kubectl apply` with a malformed entry (bare string, missing `cidr`) is rejected at admission; the union decoder no longer runs in production code paths (kept only as a test fixture documenting the legacy shape). **Sequencing constraint honored:** t003's schema tightening must not land in the same deploy as t001's normalization — normalize first, verify the fleet is clean (t002), tighten in a later deploy, per the note's "normalize first, tighten one release later" design.

## Source + Goal linkage

- **Source:** promotes `w4/019` (filed 2026-07-15 by w4/m24's simplify pass, altitude finding) via `/pm-brainstorm` round 15 — under-filed as a note: it's a deliberately sequenced, multi-task effort well over an hour.
- **Goal linkage:** platform correctness/safety — admission-time validation of a security-relevant field (IP allow-lists); w4's multi-tenant-security mandate. The shim's permanent cost: `Schemaless` + `PreserveUnknownFields` means the apiserver validates nothing — a malformed entry surfaces only as a typed-decode failure in whoever reads the CR.
- **Expected outcome:** malformed allow-list entries are rejected at `kubectl apply`/projector-write time instead of failing silently in readers; the union decoder's maintenance burden is retired.
- **Why now:** m24 shipped 2026-07-15, so the normalization step is startable immediately; every day of delay adds more legacy-shaped CRs for the backfill to chase.
- **Render parity:** omitted — the REST/GraphQL/MCP wire shape for `ipAllowList` is unchanged (that shipped in m24); this is CRD/operator-internal schema work. Coordinate with `w4/m28` (environment IP enforcement) and `w4/m24`'s shipped surfaces — neither is modified here.

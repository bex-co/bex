# w7 · m11 — Admission-time tenant-image signature verification

**Worker:** worker7 **Goal:** With tenant-image signing enabled, a pod running an unsigned or tampered image is rejected at admission — not just logged. `w6/006` shipped opt-in cosign *signing* of tenant images but explicitly deferred *verification*; three separate call-outs across two workstreams (`docs/ADR028-security-review.md`, `w7/done/m8`, `w6/done/m6`) have flagged this as deferred without ever getting an owner. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                       | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design: choose the admission mechanism (Kubernetes `ValidatingAdmissionPolicy` + CEL cosign-verify, or a lightweight admission webhook) keyed to the same signing material `w6/006`'s `BEX_TENANT_SIGNING_KEY_SECRET` uses | 40m | —          |
| t002 | Implement the admission check: reject a pod whose image fails cosign signature verification when tenant-image signing is enabled              | 1h  | t001       |
| t003 | Toggle semantics: verification enforced only when `BEX_TENANT_SIGNING_KEY_SECRET` is set — byte-identical (no enforcement) when unset, matching bex's existing optional-hardening pattern | 20m | t002       |
| t004 | Live verification: a correctly-signed image admits; a tampered or unsigned image is cleanly rejected with an explanatory event/error           | 30m | t003       |
| t005 | Docs: close the "verification deferred" call-out in `docs/ADR028-security-review.md`, `w7/done/m8`, and `w6/done/m6`'s task records            | 20m | t004       |

## Definition of done

With tenant-image signing enabled, a pod running an unsigned or tampered image is rejected at admission (not just logged), verified live; the toggle is off by default with zero behavior change when unset.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones to work on` 2026-07-13 — a codebase/doc sweep found the verification-deferred gap independently recorded in `docs/ADR028-security-review.md` ("Out of scope"), `.pm/w7/done/m8/t001.md`+`t005.md` ("recorded FUTURE-MAYBE candidate"), and `.pm/w6/done/m6/done/t002.md`, with no open milestone owning closure.
- **Goal linkage:** `GOAL.md` #7 (security review) — closes a supply-chain control that today signs but never checks.
- **Expected outcome:** tenant-image signing becomes a real admission-time guarantee instead of an inert signature nobody verifies.
- **Why now:** the gap has been flagged three separate times across two workstreams without ever getting an owner; signing without verification is security theater until this closes.
- **Render parity closing task: omitted** — pure cluster-internal admission control, no REST/GraphQL/MCP/UI surface.

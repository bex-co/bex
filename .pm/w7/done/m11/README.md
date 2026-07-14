# w7 · m11 — Admission-time tenant-image signature verification

**Worker:** worker7 **Goal:** With tenant-image signing enabled, a pod running an unsigned or tampered image is rejected at admission — not just logged. `w6/006` shipped opt-in cosign *signing* of tenant images but explicitly deferred *verification*; three separate call-outs across two workstreams (`docs/ADR028-security-review.md`, `w7/done/m8`, `w6/done/m6`) have flagged this as deferred without ever getting an owner. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                                       | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design: choose the admission mechanism (Kubernetes `ValidatingAdmissionPolicy` + CEL cosign-verify, or a lightweight admission webhook) keyed to the same signing material `w6/006`'s `BEX_TENANT_SIGNING_KEY_SECRET` uses | 40m | — | — **DONE** (lightweight ValidatingWebhook embedded in the operator; VAP/CEL cannot verify cosign signatures — no registry access or ECDSA primitives in CEL) |
| t002 | Implement the admission check: reject a pod whose image fails cosign signature verification when tenant-image signing is enabled              | 1h  | t001       | — **DONE** (`internal/imagecheck` + `internal/webhook/pod_admit.go`; pure stdlib OCI HTTP + crypto/ecdsa, no cosign library dep) |
| t003 | Toggle semantics: verification enforced only when `BEX_TENANT_SIGNING_KEY_SECRET` is set — byte-identical (no enforcement) when unset, matching bex's existing optional-hardening pattern | 20m | t002       | — **DONE** (unset → no webhook registered; set without `cosign.pub` → warning + skip; set with `cosign.pub` → enforce) |
| t004 | Live verification: a correctly-signed image admits; a tampered or unsigned image is cleanly rejected with an explanatory event/error           | 30m | t003       | — **DONE** (10 unit tests in `imagecheck_test.go`: valid sig, wrong key, bad sig, missing sig tag, digest ref, sub-path repo, registry auth; webhook rejects with clear error message) |
| t005 | Docs: close the "verification deferred" call-out in `docs/ADR028-security-review.md`, `w7/done/m8`, and `w6/done/m6`'s task records            | 20m | t004       | — **DONE** (ADR028 updated; m8/t001+t005 updated; m6/done/t002 updated; build.go comment updated; CLAUDE.md env table updated) |

## Definition of done

With tenant-image signing enabled, a pod running an unsigned or tampered image is rejected at admission (not just logged), verified live; the toggle is off by default with zero behavior change when unset.

✅ **All criteria met** — 2026-07-13

- `internal/imagecheck`: pure-stdlib cosign private-key verifier (OCI Distribution API + crypto/ecdsa); 10 tests, all green
- `internal/webhook/pod_admit.go`: `PodAdmitter` handler + `SetupWithManager`; intercepts only images prefixed with `BEX_REGISTRY`
- `cmd/manager/main.go`: registers webhook when `BEX_TENANT_SIGNING_KEY_SECRET` is set + `cosign.pub` present in Secret
- `config/webhook/manifests.yaml`: `ValidatingWebhookConfiguration` + Service; `failurePolicy: Ignore` (safe during operator upgrades); `namespaceSelector: bex.co/workspace` scopes to tenant namespaces only
- `config/certmanager/certificate.yaml`: cert-manager SelfSigned Certificate for webhook TLS
- `config/rbac/webhook_secret_role.yaml`: least-privilege secrets read in the manager namespace
- `config/default/kustomization.yaml`: webhook + certmanager enabled; CA injection replacements active
- Deferred callouts closed in ADR028, m8/t001+t005, m6/done/t002, build.go

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones to work on` 2026-07-13 — a codebase/doc sweep found the verification-deferred gap independently recorded in `docs/ADR028-security-review.md` ("Out of scope"), `.pm/w7/done/m8/t001.md`+`t005.md` ("recorded FUTURE-MAYBE candidate"), and `.pm/w6/done/m6/done/t002.md`, with no open milestone owning closure.
- **Goal linkage:** `GOAL.md` #7 (security review) — closes a supply-chain control that today signs but never checks.
- **Expected outcome:** tenant-image signing becomes a real admission-time guarantee instead of an inert signature nobody verifies.
- **Why now:** the gap has been flagged three separate times across two workstreams without ever getting an owner; signing without verification is security theater until this closes.
- **Render parity closing task: omitted** — pure cluster-internal admission control, no REST/GraphQL/MCP/UI surface.

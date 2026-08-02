# w7 · m73 — Tenant-image signature & digest integrity

**Worker:** worker7 **Goal:** bind an admitted tenant image to exactly the bytes that were signed, and stop silently deploying a mutable tag when its digest can't be resolved. **Status:** done

## Tasks (in order)

| id   | title                                                              | est  | depends_on |
| ---- | ------------------------------------------------------------------ | ---- | ---------- |
| t001 | imagecheck: parse Simple Signing payload, bind digest + repository — **DONE** | 1h   | —          |
| t002 | pinBuiltImage: fail-closed digest resolution (or Always pull) — **DONE** | 45m  | —          |
| t003 | Simplify — **DONE**                                                | 20m  | t001, t002 |
| t004 | Test coverage — **DONE**                                           | 40m  | t003       |
| t005 | Closeout — **DONE**                                                | 10m  | t004       |

## Definition of done

Admission rejects a signed payload whose `critical.image.docker-manifest-digest` or `critical.identity.docker-reference` does not match the image being admitted (mismatched-digest and mismatched-repo negative tests pass). A digest-resolution failure no longer silently deploys a mutable tag (the stale-cache risk is neutralized by Always-pull on the app container).

## Source + Goal linkage

- **Source:** codex-security scan findings #8 (medium, signature not bound to digest/repo) and #29 (low, fail-open digest resolution), validated against HEAD. imagecheck is the w7/m11 admission-verification surface.
- **Goal linkage:** Security pillar — supply-chain integrity of tenant images (ADR028; ADR013 § image verification).
- **Expected outcome:** a registry writer cannot reattach a previously-valid signed payload under another image's signature tag, and a digest-resolution failure cannot silently deploy stale/mutable bytes.
- **Why now:** #8 undermines the w7/m11 admission gate's core promise; the two ship together because both bind "what gets deployed" to a verified, immutable identity.
- **Render parity omitted:** operator admission/mechanism; no REST/GraphQL/MCP/UI surface change.

## Ship record — DONE 2026-08-01

Shipped as `ece39b92` (deployed → GitOps pin `93876eec`). `imagecheck.Verify` now parses the signed Simple Signing payload after ECDSA verification and requires `critical.image.docker-manifest-digest` == the resolved digest and `critical.identity.docker-reference` == host/repository (`verifyPayloadBinding`) — a registry writer can no longer replay a valid signature onto a different image (codex-security #8). New negative tests `TestVerify_PayloadDigestMismatch` + `TestVerify_PayloadReferenceMismatch`; the existing test helper was refactored to build payloads carrying the registry's real host so the binding has matching references. For #29, the app container sets `ImagePullPolicy: pullPolicyFor(image)` — `Always` for a mutable tag (the pinBuiltImage dev-compat fallback when the registry is unreachable), `IfNotPresent` for a digest-pinned reference — so a node can't reuse a previous App lifetime's cached gen-N tag. CI: imagecheck + full controller (incl. envtest) suites green.

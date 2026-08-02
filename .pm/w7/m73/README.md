# w7 · m73 — Tenant-image signature & digest integrity

**Worker:** worker7 **Goal:** bind an admitted tenant image to exactly the bytes that were signed, and stop silently deploying a mutable tag when its digest can't be resolved. **Status:** todo

## Tasks (in order)

| id   | title                                                              | est  | depends_on |
| ---- | ------------------------------------------------------------------ | ---- | ---------- |
| t001 | imagecheck: parse Simple Signing payload, bind digest + repository | 1h   | —          |
| t002 | pinBuiltImage: fail-closed digest resolution (or Always pull)      | 45m  | —          |
| t003 | Simplify                                                          | 20m  | t001, t002 |
| t004 | Test coverage                                                    | 40m  | t003       |
| t005 | Closeout                                                         | 10m  | t004       |

## Definition of done

Admission rejects a signed payload whose `critical.image.docker-manifest-digest` or `critical.identity.docker-reference` does not match the image being admitted (mismatched-digest and mismatched-repo negative tests pass). A digest-resolution failure fails reconciliation instead of returning the mutable `gen-N` tag as success (the fail-open regression test is removed/flipped), or generated workloads set `imagePullPolicy: Always`.

## Source + Goal linkage

- **Source:** codex-security scan findings #8 (medium, signature not bound to digest/repo) and #29 (low, fail-open digest resolution), validated against HEAD. imagecheck is the w7/m11 admission-verification surface.
- **Goal linkage:** Security pillar — supply-chain integrity of tenant images (ADR028; ADR013 § image verification).
- **Expected outcome:** a registry writer cannot reattach a previously-valid signed payload under another image's signature tag, and a digest-resolution failure cannot silently deploy stale/mutable bytes.
- **Why now:** #8 undermines the w7/m11 admission gate's core promise; the two ship together because both bind "what gets deployed" to a verified, immutable identity.
- **Render parity omitted:** operator admission/mechanism; no REST/GraphQL/MCP/UI surface change.

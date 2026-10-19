# CLI sweep 508 — `blueprints validate` size, multi-document, and `-w` dialects

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build `0952611b8`, 2026-10-19T09:09Z. Validation only; nothing created.

| probe | observed result |
| --- | --- |
| ~600 KiB manifest (comment-padded valid file) | Exit `1`, server `413 payload_too_large`: "Blueprint manifests are limited to 512 KiB" — the documented deliberate divergence from Render's 10 MB multipart limit (ADR018, w8/m48), named precisely at the CLI surface. |
| Two YAML documents in one file | Exit `1`, `valid:false`, located at the second document (`line 17, column 1`): "Blueprint must contain exactly one YAML document". |
| `-w <synthetic workspace id>` | Exit `1`, `403 FORBIDDEN` with no resource detail. |
| `-w <second accessible workspace>` | Exit `0`, `valid:true` — explicit-workspace validation works for a workspace the QA identity can access. |

## Safety and unrun checks

- Nothing created or mutated. The second-workspace probe was validate-only (no fixture, no writes).
- Unrun: a manifest just under the 512 KiB line (boundary ±1), `-w` by workspace *name* (the flag documents an ID), validate against a workspace where the identity can view but not create (no such QA workspace available).
- Deploy lag means deployed revisions can precede local `main`.

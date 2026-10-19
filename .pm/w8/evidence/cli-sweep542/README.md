# CLI sweep 542 — service rename: collision, free-form label, bounds

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD-built CLI, 2026-10-19T10:48–10:50Z. One owned fixture created, renamed repeatedly, deleted by ID (former-URL `404`).

| probe | observed result |
| --- | --- |
| Rename onto `hello-go` (baseline service, same kind) | `409 (CONFLICT): name "hello-go" is already in use`; readback unchanged, no deploy side effect. |
| Rename to the same name (no-op) | Exit `0`. |
| Rename to `Bad_Name!` | **Accepted** — and correctly so: a service update `--name` writes the free-form display label (`spec.displayName`, rendered as `name` once set), while the slug minted at create stays immutable (`slug: qa-…-rn542` throughout). The strict `RESOURCE_NAME_INVALID` rule (w5/093) binds the slug-seeding create name, not the display label; name-selector reads (`deploys list 'Bad_Name!'`) resolve the label. This matches Render, whose service names are free-form display strings over a fixed subdomain. |
| Rename to a fresh strict name | Exit `0`; slug still the original. |
| Rename to empty / whitespace-only | Client treats it as no flag at all (`at least one update flag must be provided`, upstream behavior). |
| Rename to 300 chars / 80 chars | 300 → `400 (DISPLAY_NAME_TOO_LONG): display name must be at most 100 Unicode code points`; 80 accepted and read back at length 80. |

Cleanup: exact-owned `srv-dbavb3nmnaes738pfp4g` deleted by ID (delete echo carried its final 80-char label); former-URL 404 verified. The baseline `hello-go` service was a read selector only.

Board movement during the sweep: w8/128 was formally closed (`0158b3b1a`); the w8/130 implementation is still in-flight uncommitted in this tree (untouched by this loop).

- Unrun: display-label collision semantics (two services sharing one free-form label — only the strict slug/name uniqueness is enforced), rename of a suspended service (settings-edit class, sweep 515).
- Deploy lag means deployed revisions can precede local `main`.

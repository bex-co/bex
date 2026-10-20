# CLI sweep 1153 — custom domain validation and lifecycle (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404; its custom domain died with it). No new bug filed.

- The domains list starts `[]`. Adding `qa1153.example.com` → 201 with `verificationStatus: unverified`, `certificateStatus: pending`, and the expected CNAME instruction pointing at the service's `onbex.co` host.
- Malformed names — embedded spaces, leading/trailing label hyphens, and an over-long label — all → 400. Only the well-formed domain was stored.

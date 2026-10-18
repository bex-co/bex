# CLI sweep 498 — routes/headers are static-site-only, gated precisely

**Verdict: no bug filed.** One free web service was created, probed, and deleted by ID with a HTTP `404` absence proof; the baseline was restored exactly (one new row belongs to the concurrent hunt).

## Probes (all expected outcomes)

| probe | observed result | assessment |
| --- | --- | --- |
| `GET /v1/services/{web}/routes` | HTTP `400`: `service "…" is not a static_site` | Redirect/rewrite rules are a static-site feature on Render; the gate names the exact reason. |
| `PUT /v1/services/{web}/routes` (one redirect) | Same HTTP `400` | Write path gated identically, no partial application. |
| `PUT /v1/services/{web}/headers` (one header rule) | Same HTTP `400` | Header rules share the static-site gate. |

The matching positive path on a real static site is covered in sweep 499.

## Safety and unrun checks

- No plan mutation; only the exact owned ID was deleted; secrets unrecorded.
- Deploy lag means deployed revisions can precede local `main`.

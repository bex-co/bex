# CLI sweep 1211 — preDeployCommand is not an image-runtime field (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- PATCH `serviceDetails.envSpecificDetails.preDeployCommand` on an image-backed service → 400, and the read-back envelope stays empty: pre-deploy commands belong to native runtimes, and the image envelope refuses the field rather than storing a no-op. A guessed top-level previews flag likewise → 400 (unknown field).

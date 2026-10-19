# CLI sweep 548 — digest-pinned image deploys byte-pinned

**Verdict: no bug filed.** Completes sweep 547's image-reference matrix (untagged ✓, zero-digest → named early failure ✓, this sweep: real digest). Same isolated session, `bex-canary`, HEAD-built CLI, 2026-10-19T11:10–11:11Z.

The digest of `ealen/echo-server:0.9.2`'s OCI index was resolved straight from Docker Hub's registry API (`docker-content-digest: sha256:006b92e1…4790e2`), then:

- `services create --image docker.io/ealen/echo-server@sha256:006b92e1…` → accepted, stored `imagePath` read back byte-identical (no tag re-resolution or rewriting);
- deploy `live` in ~35 s, public URL `200`;
- deleted by exact ID (`srv-dbavlae05jqs73cjk3ug`), former-URL `404`.

Pin state at sweep time: production advanced one step to `f4e38371055f` — still short of `df689525f`/`a190c54c5`/`5ae176831`, so the w8/129, /130, /131 live verifications remain queued behind the next pin.

- Unrun: digest of a single-platform child manifest (vs the index digest used here), tag+digest combined reference form.
- Deploy lag means deployed revisions can precede local `main`.

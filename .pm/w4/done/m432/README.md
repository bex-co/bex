# w4 · m432 — Rolling back a prebuilt image named by tag pulls what the tag names now, and fails if it is gone, as on Render

**Worker:** worker4 **Goal:** a rollback of a registry-image service selects the target deploy's original reference: a tag is pulled again, a digest runs the exact image, and an image that no longer resolves fails the rollback while the live release keeps serving. **Status:** done — all nine tasks done; the live DoD passed on production pin `7aaa8700befa` (2026-11-16)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Map which image reference a rollback selects today, and where the original tag or digest is kept — **DONE** | 20m | — |
| [t002](done/t002.md) | A rollback selects the target deploy's original reference: a tag is pulled again, a digest stays exact — **DONE** | 50m | t001 |
| [t003](done/t003.md) | A rollback to a tag that no longer resolves fails as a deploy, and the live release keeps serving — **DONE** | 40m | t002 |
| [t004](done/t004.md) | Dashboard rollback confirmation and the ADR matrices say what a tag rollback runs — **DONE** | 20m | t002 |
| [t005](done/t005.md) | Live check on production: tag moved, digest pinned, tag deleted — **DONE** | 30m | t003, t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t007, t008 |

## Definition of done

1. On production in `bex-canary`, t005 records: a rollback to a deploy of `:v1` after `:v1` was re-pushed serves the new image and records its digest; a rollback to a digest-pinned deploy serves that exact digest; a rollback to a deleted tag fails with an image reason while the service keeps answering. The service is deleted afterwards (by-id `GET` 404).
2. Restart of an image service still reruns the running image.
3. ADR004's rollback matrix and ADR018's Rollback row state Render's rule, with no external-image divergence left.
4. `cd lego/backend && go test ./...`, `make test` and `make lint` in `lego/operator/`, and dashboard `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for one aspect of render parity` (the Deploys aspect), 2026-11-16, item 2; filed by `/pmx them all to w4` the same day.
- **Render reference:** render.com/docs/rollbacks § Registry-hosted Docker images, read 2026-11-16: "The rollback uses the same image tag or digest as the target deploy. With a tag, Render pulls the latest image under that tag, which may differ from the original. A digest always refers to the same image. If the image is no longer available or reachable, the rollback fails."
- **bex today:** `lego/backend/internal/deploys/rollback_config.go:46` selects `target.ResolvedImage`. ADR004 (§ Saved settings and selected runtime configuration, w5/m107) records it as a "retained divergence", and ADR018's Rollback row lists it under Known limits. Neither records the user's sign-off, so under AGENTS.md ("A divergence stands only when ADR018 records it with the user's sign-off") it does not stand.
- **Related:** blocked w4/m418 changes which image a configuration-only release runs (Render reruns the running image there). Its trap note about digest selection applies here; t001 reads it first. The two rules differ on purpose and must stay separate.
- **Decision state:** following Render is this proposal's recommendation, resting on ADR008 Render parity first. It is not a user decision. Pinning the resolved digest may have been a safety choice; if t001 finds a security reason (for example signature verification bound to the recorded digest), t001 records it and the milestone stops for the user's sign-off before t002.
- **Goal linkage:** ADR008 Render parity first; ADR018 § Deploys, Rollback row; ADR004.
- **Expected outcome:** an owner who rolls back to `:stable` after re-pushing it gets the image they pushed, as on Render.
- **Why now:** rollback was just reworked (w4/652, w4/661), so the selection code is fresh; the divergence has no sign-off.
- **Render parity:** included: rollback results read on REST, GraphQL, MCP and the dashboard change.

## Unblock record (2026-11-16, loopx worker)

- **Affected:** w4/m432 t005, t009. **Source:** verified evidence, not a user decision.
- **Finding:** the release gate cleared by itself. Production runs pin `7aaa8700befa` (pin commit `546945798`, live 16:27Z), which carries 2cd28d692.
- **Outcome:** t005 ran live the same day and every Definition of done item holds (evidence in `done/t005.md`), so the milestone closed instead of returning to the open queue. The fixture registry was ttl.sh (anonymous, public, honors manifest `DELETE`), since a new GHCR package cannot be made public by API.

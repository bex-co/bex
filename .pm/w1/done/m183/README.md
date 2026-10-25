# w1 · m183 — Registry retention never deletes a live release's image

**Worker:** worker1 **Goal:** Zot's retention keeps every image a service is running (and recent rollback targets), however many newer builds were pushed, so a reschedule can always pull the live release. **Status:** done (2026-10-25)

## Tasks (in order)

| id   | title                                                           | est | depends_on |
| ---- | --------------------------------------------------------------- | --- | ---------- |
| t001 | Retention keeps `live-*` tags: canonical config + migration — **DONE**     | 30m | —          |
| t002 | Operator tags the live artifact `live-<revision>` on promotion — **DONE**  | 60m | t001       |
| t003 | Simplify — **DONE**                                                        | 15m | t002       |
| t004 | Test coverage — **DONE**                                                   | 30m | t002       |
| t005 | Closeout — **DONE**                                                        | 10m | t004       |

## Definition of done

- The canonical Zot config (and the storage migration onto existing configs) carries a second `keepTags` rule that retains `live-*` tags beyond the most-recently-pushed window.
- When a release becomes Running on a cluster-registry image, the operator pushes the same manifest under `live-<revision>`, idempotently (a status field records the protected digest; a failure retries on the next reconcile without failing the rollout).
- A test proves retention cannot select a live-tagged manifest after more than `retentionCount` newer pushes, and the operator tags exactly once per promoted digest.

## Source + Goal linkage

- **Source:** [`.pm/w1/done/139.md`](../done/139.md) (promoted 2026-10-25 by `/loopx w1`), from the 2026-10-24 beancount.io outage.
- **Goal linkage:** hosting reliability: a live service must survive any reschedule (node roll, eviction) the way it does on Render.
- **Expected outcome:** a run of builds that never go live can no longer evict the image a running service needs; rollback targets survive too.
- **Why now:** the outage already happened once; any service with more than five pushes since its last live deploy is exposed until this lands.
- **Render parity task omitted:** operator/registry-internal mechanism, no REST/GraphQL/MCP/UI surface changes.

## Closeout (2026-10-25)

- t001: canonical Zot retention gains `{patterns: ["^live-.*"], mostRecentlyPushedCount: 10}`; `setZotStorage` migrates it onto existing configs (`TestZotConfigStorageMigrationAddsLiveTagRetention`).
- t002: `protectServingImage` (`lego/operator/internal/controller/live_tag.go`) runs after `recordImageDigest` in `markRunning` and `reconcileCronJob`; `registry.TagManifest` re-PUTs the manifest as `live-<generation>` with the App's own credential; `status.protectedImageTag` makes it once per release and retries failures.
- t003: `/simplify` review applied (reuse `ownImageReference`, `baseFor`, `defaultHTTPClient`, no-redirect guard; tag not digest, so a rollback re-tags).
- t004: tag-once, retry, rollback re-tag, external and other-repository images left alone; `make test` and `make lint` green.
- Not proven by a test: Zot's own evaluation of two `keepTags` rules (documented as "a tag is retained if it meets at least one rule"; a tag matching no policy is removed). Note: `live-*` tags also occupy slots in the general 5-tag window.

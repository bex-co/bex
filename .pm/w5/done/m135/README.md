# w5 · m135 — An App's build cache lives in a repository no other App can own

**Worker:** worker5 **Goal:** No App's registry grant, revocation or deletion reaches another App's images: the build cache moves to a repository name no Kubernetes object can produce. **Status:** done

## Tasks (in order)

| id   | title                                                                | est | depends_on |
| ---- | -------------------------------------------------------------------- | --- | ---------- |
| t001 | Name the cache repository `<repo>_cache` — **DONE** | 30m | — |
| t002 | Grants and revocations leave the previous `-cache` repository's owner alone — **DONE** | 40m | t001 |
| t003 | Deletion leaves the previous name alone and survives an unhonored cache grant — **DONE** | 30m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage — **DONE** | 30m | t003 |
| t006 | Closeout — **DONE** | 10m | t005 |

## Definition of done

- An App's build cache repository is `<repo>_cache`. Kubernetes names cannot contain `_`, so no App's image repository has that name.
- With Apps `web` and `web-cache` in one workspace, `web`'s credential setup no longer grants itself `web-cache`'s image repository, so the two stop overwriting each other's Zot access rule.
- Revoking `web`'s credentials removes only `web`'s user from its previous `-cache` and legacy cache repositories, never another App's rule.
- Deleting `web` deletes its image and cache repositories, never touches the previous `-cache` name that `web-cache` may own, and completes in per-App mode both before and after Zot honors the new cache grant.
- ADR060 D3 records the new name and what the upgrade does to existing caches.

## Decisions

- **Deletion leaves the previous name alone (t003, 2026-10-07).** A first cut deleted `<repo>-cache` when no App owned it as its image repository. Two reviews found that this wedges every App deletion in per-App mode. The credential pass that precedes deletion gives the App's claim back, so after a Zot restart its credential gets 403 there, and before the restart it gets 403 on the new `_cache` grant. Deletion now never reaches for the old name, and it treats a 403 on the new cache repository as nothing to reclaim, since cache-save pushes with the same credential. A test against a fake Zot that enforces the stored grants pins both. The trial caches left under old names are w5/151.

## Source + Goal linkage

- **Source:** promoted from inbox w5/150, found by w5/m134's quality review (2026-10-07).
  - `identity.CacheRepo` is `Repo() + "-cache"` (w7/m86), which is also the image repository of an App named `<name>-cache`.
  - `registry/creds.go` `EnsureCredsFor` grants the App's user exclusive access to both. The two Apps overwrite each other's rule on every reconcile, and each overwrite resets activation, whose failed probe ends in rate-limited registry bounces.
  - `RevokeCredsFor` removes the shared entry, and `deleteRegistryRepo` deletes that repository's images. For legacy unscoped Apps the tombstone revoke removes `<name>-cache` outright, which can be another tenant's App.
- **Goal linkage:** ADR034 §6 per-App registry scope; ADR008 reliability; data integrity of tenant images.
- **Expected outcome:** two Apps named `x` and `x-cache` coexist without losing registry access, and deleting one never deletes the other's images.
- **Why now:** it is data loss, it can trigger registry-wide restarts, and w5/m134 just moved kpack out of the cache repository, so only BuildKit's cache lives there.
- **Render parity omitted:** operator-internal registry naming; no REST, GraphQL, MCP or dashboard surface changes.

## Result (2026-10-07)

- **Name (t001):** `identity.CacheRepo` is `Repo() + "_cache"`, which no App's image repository can be. `PriorCacheRepo` and `LegacyPriorCacheRepo` keep the old `-cache` names for giving claims back only. BuildKit's cache phases follow through `build.Options.CachePath`.
- **Grants (t002):** `EnsureCredsFor` grants the image and cache repositories and drops only this App's user from the old name, in one config hold (`ensureZotConfigEntryReleasing`). `RevokeCredsFor` does the same for the current, old and legacy names within its existing holds (`removeZotConfigEntryReleasing`). A shared `dropZotRepoUser` keeps another App's grant in every case. With Apps `web` and `web-cache`, the config converges and stops being rewritten.
- **Deletion (t003):** `deleteRegistryRepo` deletes the image and cache repositories, never the old name. A 403 on the cache repository, a grant Zot has not honored yet, counts as nothing to reclaim. A first cut that deleted the old name when no App owned it would have wedged every App deletion in per-App mode; see Decisions.
- **Simplify (t004):** three reviews. They found the deletion wedge, a flapping check that could not fail, a vacuous cleanup test, duplicated release logic, the lopsided `grant, release` signature, and leftover `-cache` text in a comment, `lego/operator/AGENTS.md` and a gate-off sweep.
- **Tests (t005):**
  - identity: `TestCacheRepoIsNeverAnAppsImageRepository`.
  - registry: `TestASiblingNamedLikeThePriorCacheKeepsItsGrant` (sibling's grant kept; zot-config not rewritten on a second round; revoke keeps it); `TestAnAppGivesBackItsPriorCacheGrant` (credential pass and revoke, beside another user's read grant); `TestATombstonedRevocationLeavesALegacySiblingsGrant`; the updated tombstone test.
  - controller: `TestPerAppRegistryCleanupCompletesAcrossTheCacheRename`, against a fake Zot that enforces the stored grants, after and before a Zot restart and beside a `web-cache` App.
  - 11 mutants fail them, including the shipped `-cache` name and both deletion wedges.
- **Docs:** ADR060 "Cache repository name (w5/m135)", the D3 scope line and diagram, and ADR074's naming table.
- **Gates:** operator `make test`; `make lint` (all four modules).
- **Upgrade:** each registry App writes zot-config once. BuildKit caches start cold. The new cache grants apply after Zot's next restart, and until then cache phases fail open.
- **Follow-up:** w5/151, a one-time cleanup of the trial caches left under the old names, which needs production registry access.

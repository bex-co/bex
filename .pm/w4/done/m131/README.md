# w4 · m131 — Service placement reads reflect committed moves

**Worker:** worker4 **Goal:** service detail and list responses immediately reflect committed project/environment placement without waiting for Kubernetes projection. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add a bounded authoritative service placement read — **DONE** | 40m | — |
| t002 | Use committed placement for service detail and list responses — **DONE** | 45m | w4/m131/t001 |
| t003 | Verify composed surfaces, filtering, removal, and authorization — **DONE** | 40m | w4/m131/t002 |
| t004 | Render parity and placement contract documentation — **DONE** | 20m | w4/m131/t003 |
| t005 | Simplify the changed code — **DONE** | 20m | w4/m131/t004 |
| t006 | Run regression and backend verification — **DONE** | 30m | w4/m131/t004 |
| t007 | Closeout after observable read consistency is proven — **DONE** | 15m | w4/m131/t005, w4/m131/t006 |

## Definition of done

- After committed membership updates, service Get/List use the authoritative project/environment IDs even while CR labels still show the old placement. All exposed API surfaces agree; environment filtering sees the same committed state.
- Clearing or deleting placement clears stale values. A missing managed source row is not exposed as a still-existing service; source failures are explicit. Tenant ownership and preexisting authorization remain enforced.
- Runtime status remains Kubernetes-derived, and unmanaged/storeless services retain their CR-based behavior.
- Reads are bounded to already-authorized IDs, with one batch query for a list rather than a per-service lookup or global tenant scan.
- Existing optional REST field semantics are preserved and documented. Regression tests demonstrate the old failure and new behavior, including real Postgres source reads; relevant full backend verification passes.

## Source + Goal linkage

- **Source:** promoted from w4/109; [complete live finding](source-109.md) retained. Investigation found apps.Get/List build placement from CR labels, while SetEnvironmentServices commits Postgres immediately and only synchronously projects isolation/IP settings.
- **Goal linkage:** ADR003's authoritative control-plane state and ADR006's consistent API surfaces. Clients must be able to trust a successful placement move when they read the service back.
- **Expected outcome:** immediate placement reads across REST/GraphQL/MCP without waiting for the background projector.
- **Why now:** the lag makes scripts and dashboard environment-dependent controls act on stale membership. A synchronous label patch alone remains vulnerable to a concurrent stale projector pass.
- **Scope decision:** the omission claim in109 is not a Render contract defect. The pinned service schema declares environmentId as optional string, not nullable; projectId is a bex extension. Preserve omission rather than introduce incompatible nulls. This multi-surface read correction exceeds a sub-hour note, hence promotion.
- **Render parity included:** the user-facing detail/list and filtering contract spans three APIs and the dashboard consuming GraphQL. Local integration proves read semantics; this milestone does not claim a production rollout or repeat of the timed live probe.

## Implementation and evidence (2026-09-21)

`store.GetAppPlacements` selects only requested immutable IDs in one query and returns tenant/project/environment identifiers. `apps.readViews` is shared by Get and List after their authorization/resource selection; it hydrates only managed CRs, clears old placement from empty source IDs, excludes missing rows, refuses tenant mismatches, and propagates store failures. Runtime view construction remains CR-derived. The production composition root explicitly wires the reader; unmanaged/storeless mode stays unchanged.

Native PostgreSQL regressions cover assignment, move, clear, environment/project deletion, missing/deleted app IDs, requested tenant identity, exclusion of unrequested IDs, empty input without SQL, and cancellation. Composed REST/GraphQL/MCP tests cover initially empty and stale nonempty labels, Get/List, one batched read for two services, REST filtering, REST omission versus GraphQL null, missing/foreign/error responses, authorization before any placement query, and fallback modes. Disabling authoritative hydration via a Go overlay fails all three move/clear regression cases; CR labels are intentionally unchanged throughout. Focused apps tests and the composed tests pass, including after simplify.

The three simplify reviews found no reuse/abstraction issue. Applied their small improvements: tenant/existence validation precedes discarded view construction, reader responsibility is explicit, and documentation spacing is corrected. Backend lint passes with zero issues (the first invocation encountered another session's linter lock; the normal rerun succeeded). No dashboard source was changed and no production timing replay is claimed.

## Closeout

Passed the full backend suite (`GOWORK=off go test -p 1 ./...`) with real Postgres, OpenFGA, and OpenBao KV enabled; log `/tmp/bex-w4-m131-full.log`. Post-simplify composed placement tests pass (`/tmp/bex-w4-m131-after-simplify.log`); backend lint reports zero issues (`/tmp/bex-w4-m131-lint-final.log`). The old-read-path mutation fails the three move/clear cases (`/tmp/bex-w4-m131-mutation.log`). This closes the promoted source109 implementation; the REST omission premise was disproved by the pinned schema, and its original QA history remains in `source-109.md`. No production deployment or timed replay is claimed.

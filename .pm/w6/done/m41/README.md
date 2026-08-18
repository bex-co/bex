# w6 · m41 — Causally-ordered availability edges: refuse time-traveled unhealthy conclusions

**Worker:** worker6 **Goal:** a stale operator conclusion can no longer page a tenant — an unhealthy edge is recorded only when it is genuinely newer than the last healthy checkpoint, so phantom Critical `server_failed`/`server_available` pairs stop reaching push, webhooks, and the events feed **Status:** done

## Tasks (in order)

| id   | title                                                                          | est | depends_on   |
| ---- | ------------------------------------------------------------------------------ | --- | ------------ |
| t001 | Reproduce: a time-traveled condition produces a phantom edge through the debounce | 30m | —            | — **DONE**
| t002 | Refuse an unhealthy edge older than the last recorded healthy checkpoint          | 40m | w6/m41/t001  | — **DONE** |
| t003 | Make rejected time-traveled conclusions observable instead of silent              | 25m | w6/m41/t002  | — **DONE** |
| t004 | Render parity: the availability-edge projections across every surface             | 20m | w6/m41/t003  | — **DONE** |
| t005 | Simplify the code this milestone changed                                          | 20m | w6/m41/t004  | — **DONE** |
| t006 | Test coverage: real outages unchanged, phantoms suppressed                        | 30m | w6/m41/t004  | — **DONE** |
| t007 | Closeout                                                                          | 10m | w6/m41/t006  | — **DONE** |

## Definition of done

A conclusion whose condition transition predates the last recorded healthy checkpoint records **no** edge and increments a rejection counter — proved by a test that produces a phantom pair against today's code and none after. A genuine crash-loop still records **exactly one** `server_failed` and **exactly one** `server_available`: the existing `TestPGObservedCrashEdgeEmitsExactlyOnePair` stays green unmodified, and the `w3/m78` behaviors it protects (`RolloutSettling` exclusion, `debounceUnhealthy`) are preserved rather than replaced.

## Source + Goal linkage

- **Source:** [`.pm/w3/016.md`](../../w3/016.md) (filed from `w3/m78`'s live crash leg, 2026-08-08), re-verified against HEAD by `/pm-brainstorm` 2026-08-17. [`docs/ADR052-notifications.md`](../../../docs/ADR052-notifications.md) § Consequences item 2 records the same thing as its named residual: *"multi-minute operator informer staleness (control-plane incidents) can still slip a pair through — tracked as `w3/016` (quorum-confirm hardening), not a gate."*
- **What is true at HEAD:** `w3/m78` fixed the common case twice over — the operator writes `RolloutSettling` when its live pod scan shows the full current-revision complement Ready while Deployment bookkeeping lags, and `debounceUnhealthy` (`lego/backend/internal/store/reconciler.go:211`, applied at `:301`) requires two consecutive unhealthy observations. But **both ticks read the same operator-cached conclusion**, and the reconciler holds no uncached client. When controller-runtime's informer time-travels, a debounce of two stale reads is still two stale reads.
- **Goal linkage:** [`GOAL.md`](../../GOAL.md) #2 (basic obs for operation). Each phantom pair is a **Critical push page to every policy-enabled member plus an outbound webhook delivery**, followed by a recovery ~3s later.
- **Expected outcome:** the ordering guard makes the existing debounce *correct* rather than merely doubled — at no extra read cost, since `LastTransitionTime` is already on the condition the reconciler reads.
- **Why now — the weakest of this round's why-nows, recorded honestly.** `w3/016` says "not urgent", and ADR052 records this residual as explicitly **"not a gate"**. The argument for scheduling it is timing, not urgency: `w11/m5`'s push channel is at its release-qualification gate (`t007`, physical-device evidence), and a false Critical page is the fastest way to burn a notification channel's credibility — cheaper to fix before real devices start paging than after. The user was shown this reasoning and chose to schedule it.
- **Render parity task included:** the milestone changes what lands in the composed event feed (`server_failed`/`server_available` are typed events surfaced on REST `GET /v1/services/{id}/events`, GraphQL `serviceEvents`, MCP `list_service_events`, and the dashboard) and therefore what outbound webhooks and push deliver. No field or verb shape changes, so the task is a cross-surface consistency check, not a Render diff.

## Prefer the ordering guard over a quorum re-read

`w3/016` offers two shapes. The **`LastTransitionTime` ordering guard** (t002) is preferred over a direct-quorum confirm read: it needs no new client, adds no apiserver load, and cannot itself stall. If t001's reproduction shows ordering alone is insufficient, record why before reaching for the quorum read — do not add an uncached client speculatively.

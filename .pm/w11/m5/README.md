# w11 · m5 — Push channel + notification hygiene

**Worker:** worker11 **Goal:** make push the mobile anchor with authorized durable device subscriptions, event-driven delivery, urgency and working-hours policy, safe deep links, and observable retry/pruning behavior. **Status:** todo (t001–t006, t008, t010 done; t007 physical-device qualification blocked — no signed device / production Apple Team ID / Android fingerprints available; t009 `/simplify` has no fresh diff since the m5 code shipped in prior commits; t011 closeout blocked on t007's real-device evidence)

## Gating

Starts after `w11/m2/t009`; may run in parallel with m3/m4. Reuse the existing notifications membership/service policy and composed webhook event feed. Do not duplicate `w8/001`'s usage-threshold producer or build forbidden Slack delivery.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add the durable authorized device-subscription store and service core — **DONE** | 60m | w11/m2/t009 |
| t002 | Add the provider-neutral push transport and credential configuration — **DONE** | 60m | t001 |
| t003 | Project deploy, crash, and cron events into idempotent push jobs — **DONE** | 60m | t002 |
| t004 | Implement urgency, per-service filters, timezone, and working hours — **DONE** | 60m | t003 |
| t005 | Add native permission, token registration, inbox, badge, and deep links — **DONE** | 60m | t004 |
| t006 | Add retry, stale-token pruning, audit, metrics, and privacy controls — **DONE** | 45m | t005 |
| t007 | Verify delivery, quiet hours, and deep links on real devices | 60m | t006 |
| t008 | Render parity — **DONE** | 30m | t007 |
| t009 | Simplify | 20m | t008 |
| t010 | Test coverage — **DONE** | 60m | t008 |
| t011 | Closeout | 10m | t010 |

## Definition of done

An authorized signed-in device registers and replaces its token safely, receives one visible notification for a durable deploy-failed or server-failed event, and opens the exact allowed resource/session deep link. Event filters, urgency, timezone-aware working hours, collapse/dedupe, bounded retries, invalid-token pruning, audit, metrics, and redaction are proven. Missing transport configuration disables push honestly without breaking supervision; endpoints/tokens never appear in logs or cross-workspace responses.

## Source + Goal linkage

- **Source:** ADR048 D2 and gaps 1/3; existing `internal/notifications` and outbound-webhook event pipeline.
- **Goal linkage:** the mobile alert → evidence → safe-action loop central to ADR048 and platform operational trust.
- **Expected outcome:** users are notified promptly without alert fatigue or a parallel preference model.
- **Why now:** push is the anchor, but it depends on m2's authenticated device identity and can progress independently of supervision screens.
- **Render parity:** included for shared notification settings and event semantics; native push is a documented bex delivery-channel extension, not a replacement for existing REST/GraphQL/MCP behavior.

# w1 · m167 — Preserve configured commands when cloning image cron jobs

**Worker:** worker1 **Goal:** A successful CLI clone executes the source cron command. **Status:** in progress (t001–t004 done 2026-09-29; t005 live clone waits on the deploy, then t006)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Preserve image cron commands in the Render read projection — **DONE** | 45m | — |
| t002 | Verify shared serializers and pinned clone consumers — **DONE** | 45m | t001 |
| t003 | Check REST GraphQL MCP and dashboard parity — **DONE** | 25m | t002 |
| t004 | Simplify the changed projection — **DONE** | 15m | t003 |
| t005 | Prove the clone command survives end to end | 35m | t003, t004 |
| t006 | Close out after observable acceptance | 10m | t005 |

## Definition of done

- A fresh free image-runtime cron with `--cron-command 'echo QA_CLONE_MARKER'` clones through the installed pinned CLI using `services create --from "$SOURCE_ID" --name "$UNIQUE_NAME" --region frankfurt --confirm -o json`, without repeating `--cron-command`. It exits zero and preserves the image, schedule, and exact command; a scheduled clone run logs the marker.
- Both the original read response and pinned client's generated clone POST carry `serviceDetails.envSpecificDetails.startCommand`. Runtime remains `image`; no invented repository/build settings or fake region are added.
- Native and Docker cron clones, ordinary image web services, and intentionally empty image-entrypoint commands retain their behavior. Existing authentication/authorization/not-found contracts remain unchanged.
- Meaningful regression tests exercise the pinned client's decode → clone defaults → normalization → request-building path. Echoing Bex-only raw JSON fields back to the API is insufficient.
- New fixtures are deleted by recorded ID and absence verified. Complete all tasks before moving this milestone to `done/`.

## Source + Goal linkage

- **Source:** looping user-requested `/qa-find-bugs-cli`, production sweep 3, 2026-09-22 UTC. [t001](t001.md) contains durable sanitized evidence.
- **Goal linkage:** ADR008 dependable agent-operated hosting and ADR006/ADR018 Render-compatible server contracts.
- **Expected outcome:** automation can clone an existing-image cron without silently replacing its configured work with the image entrypoint.
- **Why now:** two actual customer-CLI clones succeeded with no persisted command, and their runs reported success. A recent Docker-cron fix did not cover the image-runtime branch.
- **Parity:** included for this tenant-facing response change. The exact local upstream pin is the contract oracle; live Render was not exercised. Render's [cron documentation](https://render.com/docs/cronjobs) also describes overriding a Docker image's default command.

## Related work and scope

`w9/done/m165` fixed Docker-cron command parsing/projection; this is the separate `runtime == "image"` early return in the same serializer. Recheck that milestone's native/Docker controls. `w9/done/070` added a nine-shape readback/resend guard but omits image cron and does not simulate the pinned clone consumer. `w9/done/m93`, `068`, and `069` address other runtime/clone projection gaps. The explicit region flag follows the documented fsn1/closed-client-enum limitation (`w9/done/m56`); do not file or change that deliberate divergence. Environment/secret-file copying is not promised by the pinned clone implementation and is outside this finding.

## Cleanup and limits

Original `srv-dap1fa14dm7c7390q810` and clones `srv-dap1i4jbdpcs73f5ee30`, `srv-dap1l9p4dm7c7390q88g` were deleted after dependent runs/deploys/domains were inventoried. All three details returned 404 and the CLI service list contained only the original baseline service at 08:12 UTC. No baseline resource was mutated. Cluster-internal teardown was not inspected with admin credentials. This filing schedules a fix; no implementation is included.

## Subsequent native-service control — 2026-09-22

At 08:29 UTC, installed `bex` cloned a fresh native Go web service with `services create --from srv-dap3meh4dm7c7390q900 --name qa-20260922-c26dd9-multiline --region frankfurt --env-var MESSAGE=<HARMLESS_MULTILINE_VALUE> --confirm -o json`. Clone `srv-dap3omjbdpcs73f5ees0` retained `runtime: go`, `buildCommand: go build -o app .`, and `startCommand: ./app`. The source reached live at 08:27:12 and the clone at 08:31:58; both served HTTP 200, and the clone body preserved the configured newline, equals sign, and spaces exactly. This is a passing native-web clone control, not a native-cron control or a fix to the image-cron bug. Both fixtures were deleted in clone/source order; detail, list, and former runtime absence were verified afterward.

## Scheduling history

Transferred intact from w9/m167 to w1/m167 by user-approved `/pm all for w1` on 2026-09-28. Original QA evidence and related w9 completion history remain authoritative. No implementation task was completed by this transfer.

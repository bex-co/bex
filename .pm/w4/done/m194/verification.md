# m194 verification — 2026-10-11 UTC

Shipped fix: `4275834fa` (deploy.yml run 38112771557 succeeded: all test jobs, build, deploy). govulncheck on that commit is red from the pre-existing golang.org/x/net advisories, unrelated.

## Change (t001, t002, t005)

- `build_strategy.go`: `cronStartCommandOverrides(spec)` — a cron whose build reads a docker command (image or Dockerfile; `readsDockerCommand(buildStrategy)`), the same set the operator's `cronPodSpec` runs `startCommand` for when `command` is empty. Native and buildpack are excluded.
- `cronCommand(spec)` feeds `view()`'s `Command`, so REST (`serviceDetails.command`, `envSpecificDetails.startCommand`/`dockerCommand`), GraphQL `command`, MCP (same render/get paths) and the dashboard show the configured fallback.
- `setCronCommand(spec, cmd)` trims; an empty result also clears a non-native `startCommand`. Used by `SetCronJob` and the cron registry probe. `setStartCommand` folds the cron/non-cron branch shared by `SetCommands` and its probe (`setStartCommand` GraphQL alias, REST `envSpecificDetails.startCommand|dockerCommand`). Omitted/null keeps both slots, and a nonempty write leaves the fallback in place behind the winning Command.
- Operator unchanged: `releaseRecordSpec` records both cleared slots, `projectRuntimeApp` applies them, `cronPodSpec` sets no container command. Blueprint `applyBlueprintCommand` is left as it is: on a Blueprint clear it clears both slots, as w4/174 intended.
- ADR038 Command bullet records the fallback read and clear semantics. Render parity: blank Docker Command means the image default, the same as Render's cron Docker Command. No divergence remains. w8/072 (upstream CLI builder) stays separate.

## Caller audit (t002)

| Caller | Disposition |
| --- | --- |
| SetCronJob: GraphQL `updateCronJob`, settings registry | clear removes both slots (tested) |
| SetCommands: registry, GraphQL `setStartCommand`, `setBuildCommand` | cron clear removes both; build-only passes nil start (unchanged) |
| Registry probes `settings.go` cron + commands entries | same transition as applied writers |
| AppView producers (12 `s.view`) | all go through `view()`; the change applies only to crons with a non-native fallback |
| Web/private/worker StartCommand | unchanged (test control) |
| Native/buildpack cron | read stays empty Command; clear keeps baked `startCommand` (backend + operator tests) |

## Automated gates (t006)

- `lego/backend/internal/apps/cron_command_fallback_test.go`: GraphQL and REST reads of the image and Dockerfile fallback. Clears through SetCronJob, SetCommands (whitespace), GraphQL `updateCronJob`/`setStartCommand`, and REST PATCH `command`, `envSpecificDetails.startCommand` and `dockerCommand` (×image/Dockerfile). Also covers schedule-only preservation, nonempty precedence, the native clear control and the web control. Without the fix, 18 subtests fail.
- `lego/operator/internal/controller/cron_command_fallback_test.go`: an unedited fallback still runs. The release recorded from a cleared spec projects no override, and its template has no command. A native cron gets no shell override.
- Backend `go test -p 1 ./...` with real Postgres/OpenFGA/OpenBao (`BEX_TEST_REQUIRE_DEPS=1`, locally and via ship-gates): pass. `make test` (operator, envtest): pass. `make lint` (4 modules): 0 issues. Dashboard unchanged.
- **Unverified:** failed or conflicting clear rollouts and rollback, scheduled execution, Dockerfile/native live runs, MCP live calls, CLI round-trip.

## Live acceptance (t003) — workspace `tea-d98210cbbpdc73dcrkvg`

Fixture `qa-20261010-m194-cron` `srv-db5htbigccos73bibni0` was created at 05:29:17Z via `POST /v1/blueprints/deploy`, using finding.md's manifest with `qa-m194-*` markers. The create response already had `command` equal to the fallback. It was Live at rev-1, and GraphQL `command` = `startCommand` = the script.

1. On a fresh Settings load, the Command input showed `sh -c 'echo qa-m194-start; sleep 60; echo qa-m194-complete'` (`.playwright-mcp/qa-m194-command-initial.png`).
2. Replay 1:
   - 05:30:13.011Z `UpdateCronJob` with the harmless command returned it.
   - 05:30:14.946Z `command:""` returned `""`.
   - `dep-db5htpigccos73bibnn0` went Live at rev-3 with `command:""` and `startCommand:""`, and the field was empty after reload.
   - Run `crr-gh37ueqojf0iaeapjqr3` ran 05:31:03Z→05:31:09Z and was successful. App logs for 05:30:30Z onward were `[]` with `hasMore:false`, so there was no qa-m194 marker and the image default ran (it was 64s before the fix).
3. Replay 2 on a fresh page:
   - 05:31:38.188Z replacement, then 05:31:39.745Z `command:""`. The field was empty after reload.
   - `dep-db5hueqgccos73bibnqg` went Live at rev-6 with both slots empty.
   - Run `crr-ulgaatg7l0l6soom69sb` ran 05:32:06Z→05:32:12Z and was successful, with logs `[]`.
4. Nonempty control:
   - 05:32:40.996Z `sh -c 'echo qa-m194-replacement-control'`.
   - `dep-db5huu2gccos73bibns0` went Live at rev-8 with `startCommand:""`.
   - Run `crr-40jd94mk5q2bilkrdtrr` ran 05:33:07Z→05:33:12Z and was successful. Its only log line was `qa-m194-replacement-control`, with `hasMore:false`. Activity shows run started/finished as Succeeded and every deploy Live or superseded.
5. Cleanup:
   - DELETE at 05:33:45Z returned 204, and a later GET returned 404.
   - Six-family inventory: postgres 4, key-value 2, env-groups 1 and blueprints 1 match the baseline. Services gained one foreign fixture, `qa-20261010-a154-idle-1m` (another worker, created 05:29:30Z, preserved). Projects lost one to another worker. No owned residue.
   - `qa-login.sh --logout` printed ok. The old jar then got whoami 401, browser cookies were 0 at about:blank, and the state/jar were removed.

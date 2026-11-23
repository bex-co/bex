# w32 · m572 — A deploy with a pre-deploy command narrates the step as Render's log does: `==> Starting pre-deploy: <command>`, then `==> Pre-deploy complete!` or `==> Pre-deploy has failed` and the exit status

**Worker:** worker32 **Goal:** a deploy's log (`bex logs --type build`, the deploy page, the live tail) narrates its pre-deploy step in Render's words, each line at a moment the operator recorded. `==> Starting pre-deploy: <command>` comes when the step starts. On a native runtime, `==> Running '<command>'` comes when the step's container starts. The command's own output follows. The step then closes with `==> Pre-deploy complete!`, or with `==> Pre-deploy has failed`, `==> Exited with status N` and the troubleshooting line. **Status:** blocked (t001–t007, t010 and t011 done and shipped as `b91b05c35a`; t008 waits for the production pin to carry it and for the user to agree to the cost of one `starter` service; t009 and t012 follow)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The operator records the step's command, its launch, its container's start, its exit status and sub-second start and end moments in `status.preDeploy`](done/t001.md) — **DONE** | 1h30m | — |
| t002 | [The control plane projects the record onto the deploy row and into `DeployProgress`](done/t002.md) — **DONE** | 1h30m | t001 |
| t003 | [`==> Starting pre-deploy: <command>` at the step's start, and `==> Running '<command>'` when a native step's container starts](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [`==> Pre-deploy complete!` at a passing step's end, before the rollout's lines](done/t004.md) — **DONE** | 30m | t003 |
| t005 | [A failed step closes with `==> Pre-deploy has failed`, `==> Exited with status N` and the troubleshooting line](done/t005.md) — **DONE** | 45m | t003 |
| t006 | [The live build tail carries the step's lines as they are recorded](done/t006.md) — **DONE** | 30m | t004, t005 |
| t007 | [Record the lines in the progress-line contract and ADR018](done/t007.md) — **DONE** | 20m | t004, t005 |
| t008 | [Ship and verify on production](t008.md) | 45m | t006, t007 |
| t009 | [Render parity](t009.md) | 30m | t008 |
| t010 | [Simplify](done/t010.md) — **DONE** | 15m | t009 |
| t011 | [Test coverage](done/t011.md) — **DONE** | 30m | t009 |
| t012 | [Closeout](t012.md) | 10m | t010, t011 |

## Definition of done

- On production, once the pin carries this milestone's commits, the released `bex` reads these logs in `bex-canary`:
  - A paid native service whose pre-deploy command exits 1. `bex logs -r <srv> --type build -o text` shows `==> Build successful 🎉`, `==> Starting pre-deploy: <command>`, `==> Running '<command>'`, the command's output, `==> Pre-deploy has failed`, `==> Exited with status 1` and `==> Common ways to troubleshoot your deploy: https://render.com/docs/troubleshooting-deploys`, in that order, with no `==> Deploying...` anywhere.
  - The same service with a command that passes. Its log shows `==> Pre-deploy complete!` after the command's output, then the rollout's own lines (`==> Deploying...` and the rest from w32/m560), then its verdict.
  - REST, the subscribe stream, GraphQL, MCP and the dashboard's deploy page read the same lines.
  - Every fixture is deleted by ID, and each delete is confirmed by a 404.
- Each line is stamped with a moment the operator recorded: the step's start, its container's start, or the step's end. `==> Starting pre-deploy` never sorts before the build's own last line. The step's closing line never sorts before the command's own last line.
- An out-of-memory kill, a step that never started (an image that cannot be pulled, no build minutes left, a program the runtime could not launch, an infrastructure fault) and the 30-minute window keep their own wording. A failure no Render log shows keeps bex's `==> Pre-deploy failed: <reason>`. The window's lines follow jdegreef/open-word-bible#1023 only if that log gives their exact wording.
- Unit tests cover each line's presence and absence by outcome (passed, failed with an exit status, out of memory, never started, timed out, canceled while it ran), by launch (native, Dockerfile, prebuilt image) and by service type (web, private service, background worker; a static site and a cron job run no step). An envtest covers the operator's record, and a real-database store test covers its projection.
- `docs/render-artifacts/live-deploy-following.md` § Platform progress-line contract and ADR018 § Logs, Build logs list the lines with the Render logs they come from.

## Source + Goal linkage

- **Source:** inbox note [w32/087](../../done/087.md), filed 2026-11-22 from w32/085's search of public Render logs, and promoted the same day by `/loopx w32`.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)) in the deploy log, at the step that most often fails a paid deploy (a migration). Render's log names the command it starts, shows the output, and closes the step with its own verdict and the command's exit status. bex's log shows the output with no opening line, and closes a failure with `==> Pre-deploy failed: <reason>`.
- **Expected outcome:** a bex deploy log with a pre-deploy step reads like Render's from the build's end to the verdict. A user whose migration fails sees which command ran, its output and its exit status, without opening the pre-deploy filter.
- **Why now:** w32/m560 gave the rollout Render's lines, and w32/085's search settled the pre-deploy lines it had left open. The note's own scope check needed the step's command and exit status on the deploy row, and moments precise enough to order the lines around the command's output. That means an operator record, a migration and the narration: about 8h over twelve tasks, so it is a milestone.
- **Render parity task included:** the lines are a tenant-facing surface (REST, SSE, GraphQL and MCP log reads, the CLI and the dashboard's deploy page).

## Render's lines (public logs, w32/085's search, 2026-11-22)

- **A step that fails.**
  - Unwrenchable/ATOMIC-FIZZ-CAPS-VAULT-77-WASTELAND-GPS#82 (2026-02-03, native Node, with timestamps): `==> Build successful 🎉` at 11:29:11.25, `==> Starting pre-deploy: Node.js` at 11:29:14.99, `==> Running 'Node.js'` at 11:29:27.17, bash's `command not found`, `==> Pre-deploy has failed` at 11:29:31.49, `==> Exited with status 127` at 11:29:32.47, and the troubleshooting line at 11:29:32.53. No `==> Deploying...` comes before the step.
  - dukens11-create/Flupflap#206 (2026-05-12): `==> Build successful 🎉`, `==> Starting pre-deploy: <command>`, `==> Running '<command>'`, Prisma's output, `==> Pre-deploy has failed`, `==> Exited with status 1`, the troubleshooting line.
  - stranske/learning-management-system#191 (2026-05-31), ganjasan/mapsurvey#72 (2026-08-15), healthkey-ai/promop#1636 (2026-09-29) and briankeane/alloy-client-portal#8 (2026-05-09) have the same shape, with exit statuses 127, 2, 127 and 128.
- **A step that passes** ends `==> Pre-deploy complete!` (BerriAI/litellm#9549, 2025-03-26; ganjasan/mapsurvey#73, 2026-08-15).
- **A step killed at Render's 30-minute limit** reads "Pre-deploy has failed / Timed out" (jdegreef/open-word-bible#1023, 2026-11-17). t005 reads that log for the exact lines (below: it quotes none).
- **Not observed:** what follows `==> Pre-deploy complete!` (w32/085, which waits for a Render account), whether a Dockerfile or prebuilt image's step gets `==> Running '<command>'`, and an out-of-memory kill. t003 and t005 look for a log that settles each one and record what they find. Until one does, `Running` follows w32/m560's start-command rule (native runtimes only), and the out-of-memory kill keeps bex's line.

### What t003 and t005 found (2026-11-22, about 700 more public issues and PRs, each runtime read from the repository's `render.yaml` at the commit)

- **A Dockerfile or prebuilt image's step has no `Running` line: settled.** ClipFarmVB/ClipFarm#173 (2026-08-07, Docker): `==> Starting pre-deploy: sh -lc 'cd /app/api && alembic upgrade head'`, sh's `not found`, `==> Pre-deploy has failed`, `==> Exited with status 127`. ganjasan/mapsurvey#72 (2026-08-15, Docker) reads the same with status 2. BerriAI/litellm#9549 (image): the command's output straight after the start line, then `==> Pre-deploy complete!`. briankeane/alloy-client-portal#8 (2026-05-09, image) and khusenov/backend-boilerplate-python#16, Mumbo-Jumbo-3/root-cause-backend#8 have no `Running` line either. Native logs have it and run the command under bash (stranske/learning-management-system#191: `bash: line 1: alembic: command not found`, status 127).
- **The 30-minute limit: not quoted in jdegreef/open-word-bible#1023.** The PR has no comments and only paraphrases "Pre-deploy has failed / Timed out". Per the definition of done, the window keeps bex's line. MalaysiaKaki/astrail-webmcp#10 (2026-10-03, Docker) shows `==> Pre-deploy timed out` about 33 minutes after the last output line, lowercase, with nothing after it in the excerpt: filed as w32/091.
- **A program that cannot be launched:** briankeane/alloy-client-portal#8 (image, `preDeployCommand: release`) reads `==> Starting pre-deploy: release`, `==> Pre-deploy has failed`, `==> Exited with status 128`, with no output. The definition of done keeps bex's line for it: filed as w32/091.
- **What follows `==> Pre-deploy complete!`:** still not settled. ganjasan/mapsurvey#73 (Docker) quotes `==> Pre-deploy complete!` at 15:57:51, the app's `Listening at` line at 15:58:21 and `==> Your service is live 🎉` at 15:58:29, shortened by its author. bex keeps `==> Deploying...` after the step; w32/085 captures it.
- **An out-of-memory kill and an image pull failure:** no public log. Both keep bex's line.

## bex today (`c01e2f55c3`)

- `progressLines` (`lego/backend/internal/logs/progress.go:367`) writes no line when the step starts or passes. A failed step closes with `terminalLine`'s `==> Pre-deploy failed` plus `: <failureReason>` (`progress.go:853`). `rendersVerdict` (`progress.go:763`) keeps bex's line because "Render's lines for a pre-deploy failure … are not observed". They are observed now.
- The step's output already reaches `type=build` reads: Alloy ships the pre-deploy Job's stdout into Loki as build lines (`LogTypePreDeploy`, `lego/backend/internal/logs/service.go:55`, w5/m100).
- `status.preDeploy` (`PreDeployStatus`, `lego/types/v1alpha1/app_types.go:1663`) records the Job, the generation, the status, a message, and the start and end in whole seconds (`time.RFC3339`). The start is the operator's first reconcile of the step (`reconcilePreDeploy`, `lego/operator/internal/controller/app_controller.go:7648`). The end is the Job's completion (`jobFinishedAt`). Neither orders a line against the pod's nanosecond-stamped output.
- The exit status is only inside the message's reason sentence (`DeployReasonPreDeployExited`, "the pre-deploy command exited with code {code}", `lego/types/v1alpha1/deploy_reasons.go:243`, from `predeploy.FailureMessage`, `lego/operator/internal/predeploy/predeploy.go:416`).
- The deploy row keeps only `pre_deploy_status`. The `pre_deploy_started` and `pre_deploy_ended` facts carry the whole-second moments (`preDeployLifecycleFacts`, `lego/backend/internal/store/reconciler.go:1579`). Nothing keeps the command once the App's status moves on to a newer release.
- `==> Deploying...` already comes after the step. The operator first writes the serving template once the step has passed (w32/m560's `status.releaseRollout`).

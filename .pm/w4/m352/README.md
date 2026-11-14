# w4 · m352 — A deploy cuts every open shell and SSH session within a minute and refuses new ones as "no running instance yet", while the instance keeps running and serving until the new release goes live

**Worker:** worker4 **Goal:** a shell or SSH session lives as long as the instance it is attached to. A deploy's build, pre-deploy and rollout leave the serving instance's sessions open, and new sessions can open on it. A session ends when its instance stops, or when the caller loses access to it. Any refusal or end names its real reason in the dashboard. **Status:** todo (t001–t005 done; t006 live production replay open)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Gate shell and SSH on a Ready instance of the serving release, not on the App phase — **DONE** | 45m | — |
| [t002](done/t002.md) | Say why a shell was refused or ended: the refusal's own reason, and a classified end reason from the gateway — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Render parity — **DONE** | 15m | t002 |
| [t004](done/t004.md) | Simplify — **DONE** | 15m | t003 |
| [t005](done/t005.md) | Test coverage — **DONE** | 30m | t003 |
| [t006](t006.md) | Closeout (live replay on production) | 30m | t004, t005 |

## Definition of done

All on production, on a Starter image web service (`docker.io/library/nginx:1.27-alpine`, created with `POST /v1/services`) whose `preDeployCommand` is `sleep 150`:

1. **An open shell survives a deploy until its instance stops.**
   - Open Shell → Web Shell (Any ready instance) and run `hostname; while true; do date -u +%T; sleep 5; done`.
   - `POST /v1/services/{id}/deploys` within 5 s of connecting.
   - Through `pre_deploy_in_progress`, at least the first two revalidation ticks (connect + 60 s and + 120 s), the loop keeps printing, and the status stays "Connected".
   - The session ends only once the deploy is live and the attached instance has left `GET /v1/services/{id}/instances`.
2. **A new shell opens during a deploy.** During `pre_deploy_in_progress`, opening the page or pressing Reconnect connects to the serving instance (the one `/instances` lists from before the deploy). GraphQL `createShellSession` and REST `POST /v1/services/{id}/shell-ticket` mint a ticket, not `409 … no running instance yet`.
3. **The refusals that should stay, still do,** each with its own reason shown in the Web Shell panel, not "Couldn't start a shell session.":
   - suspended;
   - Free plan;
   - a first deploy that has never gone live: `the service has no running instance yet; wait for the current deploy to go live`.
4. **An ended session says why.** Suspending the service while a shell is open ends it within a minute, and the panel names the suspension, in en and zh. It does not show "Connection error" or the raw `session closed`.
5. **The URL never drops.** `curl -s -o /dev/null -w '%{http_code}' https://<name>.onbex.co/` every 2 s answers `200` throughout 1–2, as it did in round 226.
6. **Fixtures.** Every fixture is deleted, and each GET answers 404.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 226, 2026-11-12 19:39–20:00 UTC (`muse.env`, workspace `bex-canary`; pin `33e1b1e6117c`). The probes are under § Finding. Screenshots: `.playwright-mcp/qa-r226-shell-1.png` (a working session) and `.playwright-mcp/qa-r226-shell-2.png` (the cut session during pre-deploy, header "Service Deploying · Pre-Deploy In Progress").
- **Governing docs:**
  - [ADR035](../../../docs/ADR035-ssh.md) — SSH and the Browser Web Shell. Line 57 says "the gateway still requires Running phase, an active revision, and a Ready current-image pod before authenticating", and § Browser Web Shell repeats "Running with a live revision" for the ticket mint. The fix amends both: the gate becomes an active revision and a Ready current-image pod, which is what "an already-running instance" means.
  - [w4/506](../done/506.md) (done) gave each refusal its own sentence. This milestone stops one of them, "no running instance yet", from firing over a running instance.
- **Goal linkage:** Render parity for SSH and Shell. Render's CLI "verifies a live deploy" before it connects (ADR035 § Context). A service whose newer deploy is building or in pre-deploy still has that live deploy, and its instance keeps serving. With zero-downtime deploys, Render keeps the old instance until the new one is healthy.
- **Expected outcome:**
  - a team with auto-deploy can keep a shell open, or open one, while a push builds or runs pre-deploy;
  - a manual command running in the shell (a migration, a `psql` session, a data fix) is not killed mid-run by an unrelated deploy;
  - the panel's wording matches what happened.
- **Why now:** major. Every deploy — and with auto-deploy that is every push — ends every open shell and SSH session at the next one-minute revalidation tick, without warning, and with a false reason. The cut kills whatever the session was running, while the instance it ran on keeps serving for the rest of the deploy. Native SSH and `render ssh` go through the same resolver.
- **Render parity task included:** the ticket mint is a REST + GraphQL surface, native SSH is the Render CLI's transport, and the dashboard copy changes.

## Finding (round 226)

Fixture `qa-20261112-r226-shell` (`srv-dbr1c4q585js73afghj0`), `POST /v1/services` at 19:40:03: Starter `docker.io/library/nginx:1.27-alpine`, first deploy live at 19:41:14. Deleted at the end of the round.

### The shell itself works

- Web Shell → "Connected" in a few seconds.
- `pwd; whoami; echo SVC=$RENDER_SERVICE_NAME ID=$RENDER_INSTANCE_ID PORT=$PORT; env | grep -c RENDER; tty; stty size` printed:
  - `/`, `root`;
  - `SVC=qa-20261112-r226-shell ID=srv-dbr1c4q585js73afghj0-0ldjjna7r8695s7irpmi PORT=80`;
  - `10`, `/dev/pts/0`, `25 104`.
- `sleep 30` then Ctrl-C printed `^C`, and `echo $?` printed `130`.
- An idle session survives revalidation ticks when no deploy runs (run 2, below: the connect + 60 s tick at 19:50:51 passed).

### A deploy cuts the session at the next tick, while its instance keeps serving

- **Run 1.** Connected at about 19:41:23; `POST …/deploys` at 19:42:17 (`dep-dbr1d6a585js73afghkg`). The panel turned "Connection error" + `session closed` at 19:42:23, the deploy went live at 19:42:54.
- **Run 2.** Connected 19:49:51; `preDeployCommand` set to `sleep 150` (PATCH, 200). Deploy `dep-dbr1h9aqljrc73co7b1g` at 19:51:01, `pre_deploy_in_progress` from 19:51:24. The session closed at 19:51:52, connect + 120 s, the second tick; the first tick, before the deploy, had passed.
- **Run 3 (the clean one).**
  - Connected at 19:55:35, on pod `…-84d47cc6b-75ttz` (instance `…b4hvr3`, created 19:53:57).
  - The loop `hostname; while true; do date -u +%T; sleep 5; done` printed every 5 s.
  - `POST …/deploys` at 19:55:40.786 (`dep-dbr1jf2qljrc73co7b40`). Its status was `created`, then `pre_deploy_in_progress` from 19:56:12.
  - The last line printed was `19:56:35`. At 19:56:36 (connect + 61 s, the first tick) the panel read "Connection error · Reconnect", and `session closed` under the terminal.
  - At that moment, `GET /v1/services/{id}/instances` still listed `…b4hvr3`. It stayed listed through 19:56:42, while pre-deploy kept running.
  - `curl https://qa-20261112-r226-shell.onbex.co/` every 2 s, 19:55:22–19:58:40: `200` every time.
- **Reconnect is refused with a false reason.**
  - At 19:56:37 the panel's Reconnect sent GraphQL `createShellSession`. The response: `{"data":{"createShellSession":null},"errors":[{"message":"conflict: the service has no running instance yet; wait for the current deploy to go live",…,"path":["createShellSession"]}]}`.
  - The panel showed only "Connection error" and "Couldn't start a shell session."
  - The service header meanwhile read "Service Deploying · Latest deploy Pre-Deploy In Progress · Instances 1".

### Mechanism (code read at `origin/main`, unchanged since the pin)

- **The gate.** `sshSessionReady` (`lego/backend/internal/apps/service.go:1332-1344`) refuses `v.Phase != Running || v.Revision == "" || v.Image == ""` (`:1340-1341`) with "the service has no running instance yet; wait for the current deploy to go live".
  - The App phase describes the latest deploy's progress: `Building` and `Deploying` while a deploy runs (`lego/types/v1alpha1/app_types.go:1390-1394`), and back to `Running` after the deploy goes live, fails or is canceled over a serving release (`:1395-1416`).
  - `Revision` is `Status.ActiveRevision`, "currently serving traffic" (`app_types.go:1891`). `Status.Image` is "the OCI image of the active revision" (`:1895`), which "remains the image of ActiveRevision while a new artifact waits on pre-deploy or rollout health gates" (`:1951-1953`). Both still describe the serving release during a deploy.
  - So the phase clause is the only one that turns false over a running instance.
- **The targets.** `readySSHInstances` (`service.go:1844-1884`) already selects only Running, Ready pods whose revision label is `ActiveRevision` and whose image is `Status.Image` (`readyCurrentAppPod`, `:1886`). The serving release's pods pass it during a deploy, and build and pre-deploy pods never do.
- **The gate's two callers** (`grep -rn "sshSessionReady" lego/backend --include='*.go'`, outside tests):
  - `ResolveSSHSession` (`:1729`);
  - `CreateShellSession` (`:1803`), which backs REST `POST /v1/services/{id}/shell-ticket` and GraphQL `createShellSession`.
- **`ResolveSSHSession` runs at connect and again every minute**, 6 call sites outside tests:
  - Web shell: at connect (`lego/backend/internal/sshgateway/webshell/websocket.go:221`) and from the revalidation watchdog (`:276-280`). `sshgateway.WithRevalidation` re-runs it every `BEX_SSH_REVALIDATE_INTERVAL`, default `core.DefaultStreamRevalidateInterval = time.Minute` (`lego/backend/internal/core/revalidate.go:28`). Production sets no override (`grep -rn BEX_SSH_REVALIDATE_INTERVAL deploy` is empty).
  - Native SSH: at connect (`nativessh/server.go:219`) and from `reauthorize` (`:583-607`, via `:452` and `:512`). That covers raw `ssh srv-…@ssh.bex.co` and `render ssh`.
  - `CompositeResolver` (`sshgateway/composite.go:45`) routes `srv-…` usernames to it.
- **Any check error cancels the stream** (`core/revalidate.go:52-55`). The web shell then sends `ws.fail("session closed")` (`websocket.go:370-373`), which records the session as `revoked` (`:286-287`).
- **The dashboard.**
  - Any `createShellSession` error other than `SHELL_UNAVAILABLE` becomes the generic "Couldn't start a shell session." (`dashboard/src/features/services/components/web-shell-terminal.tsx:92-104`, `services.shellErrorGeneric`). The server's own sentence is dropped.
  - A gateway `error` frame shows status "Connection error" (`services.shellErrorStatus`) with the frame's raw English text, here `session closed` (`:166-174`).

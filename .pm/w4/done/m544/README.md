# w4 · m544 — An agent session runs the Python its repo asks for, chosen by Render's rules, from the official builds with GNU readline

**Worker:** worker4 **Goal:** a cloud agent session picks its Python the way a Render native build does (`PYTHON_VERSION` → `.python-version` → Render's dated default), from the same table the hosted-app build uses, and runs that version from the official Docker Python builds (GNU readline) in every shell and setup step. **Status:** done

## Tasks (in order)

| id   | title                                                                             | est | depends_on       |
| ---- | --------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Move the Python version table and resolver into `lego/types/toolchains` — **DONE** | 1h  | —                |
| t002 | An `agent-toolchain` resolver binary in the agent image — **DONE** | 45m | t001             |
| t003 | Pass the session's creation time to the driver — **DONE** | 30m | —                |
| t004 | Bake each official Python line into the agent image — **DONE** | 1h  | t001             |
| t005 | The driver selects the session's Python after checkout, before setup — **DONE** | 1h  | t002, t003, t004 |
| t006 | An exact patch the image lacks is fetched on demand, with a named fallback — **DONE** | 1h  | t005             |
| t007 | Every shell sees the selected Python; overlay, ADR and README — **DONE** | 45m | t005, t006       |
| t008 | Render parity: Render's selection rules, Devin's agent-side behavior — **DONE** | 30m | t007             |
| t009 | Simplify — **DONE** | 20m | t008             |
| t010 | Test coverage — **DONE** | 30m | t008             |
| t012 | Docker Hub's blob host joins the registries catalog, so an exact patch's fetch reaches it — **DONE** | 20m | t006 |
| t011 | Closeout — **DONE**                                                               | 15m | t009, t010, t012 |

## Definition of done

On production agent sessions (a fresh session and one booted from a snapshot behave the same):

- A repository whose root `.python-version` says `3.13` gets `python3 --version` = the table's 3.13 patch (3.13.18 today) and `python3 -c 'import readline; print(readline.backend)'` = `readline`, in the agent's shell, in a setup step and in the web Shell.
- A `PYTHON_VERSION=3.11.9` secret on the repo or workspace wins over the file and yields exactly 3.11.9.
- A repository with neither gets Render's dated default for the session's creation time (3.14.10 since 2026-11-03).
- An unsupported request (e.g. `2.7.18`) fails setup by name (`AGENT_SESSION_SETUP_FAILED` naming the version), never silently picks another.
- A repository whose uv config sets `python-preference = "only-managed"` still gets uv's own python-build-standalone download, as in its CI, and `/usr/bin/python3` stays Debian's 3.11.
- The transcript shows the choice as a step: `==> Using Python <version> (from PYTHON_VERSION | .python-version | default)`.

## Source + Goal linkage

- **Source:** user decision 2026-11-20 in the automation triage session: the sandbox picks Python with Render's rules, reusing the hosted-app build's resolver (`ResolveNativeVersion`), from the official images, and leaves a repository's own `only-managed` alone. Two research passes the same day (Render's docs, the bex code, uv's docs, Devin's docs, and measured image sizes) are summarized in the task files.
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md)) run a repository's own toolchain, and the hosted-app build already follows Render's version rules ([ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md)); one table keeps the two from drifting.
- **Expected outcome:** today every session gets the baked 3.12.16 (or uv's download) whatever the repository asks for (w10/m513); after this, an agent runs, tests and debugs on the Python the repository's deploy would get on Render.
- **Why now:** agent automations run on Python repositories every few hours, and a session on the wrong line produces work that fails on the line the repository actually uses. The resolver is still operator-internal (`lego/operator/internal/build/nativeversion.go`), so the move to `lego/types` must come before any session-side caller.
- **Render parity:** included (t008). Selection follows Render's documented rules; the agent-side behavior (a visible step before setup, the PATH reaching setup steps and terminals) is checked against Devin, the reference for agent surfaces (ADR047).

## Parked 2026-11-21

t001–t010 shipped in `344463497`. What is left is t011: replaying the definition of done on production agent sessions, fresh and booted from a snapshot, once the platform pin carries that commit (the agent image is built and its build check, which this host could not run, proves the baked lines on that deploy). Then, in a repository session on the QA workspace: a root `.python-version` `3.13` gives `python3 --version` 3.13.18 and `readline.backend == "readline"` in the agent's shell, a setup step and the web Shell; a `PYTHON_VERSION=3.11.9` repo secret gives exactly 3.11.9 (its value shows redacted in the transcript, ADR098 D6); a repository with neither gets 3.14.10; `2.7.18` fails `AGENT_SESSION_SETUP_FAILED` naming it; a uv `python-preference = "only-managed"` repository still downloads uv's build while `/usr/bin/python3` stays 3.11; the transcript's first step reads `==> Using Python <version> (from …)` or `(default)`. Record the evidence here and close with t011.

## Unblocked (2026-11-22)

- **Gate cleared (verified evidence).** The newest pin on `origin/main` is `5330be9f54 chore(deploy): pin platform images to 19fdb46f1758`, and `git merge-base --is-ancestor 344463497 19fdb46f1758` holds, so production's bex-api and agent image carry t001–t010. No gate remains and no user decision is needed.
- **Next.** A `/loopx w4` worker replays the definition of done on production, as § Parked lists, in the QA account's `tian-personal` on fixture repositories under its `puncsky` GitHub installation that it deletes afterwards: a phase-1 session (no snapshot) and one booted from the repository environment's build, for each case. An agent session has no browser web Shell (w10/027), so the interactive `ssh <session>@ssh.bex.co`, the web Shell's own `/bin/sh` tty exec, stands in for it, as in w10/032. It closes with t011 if the definition of done holds.

## Live check (t011, production, 2026-11-22)

Workspace `tian-personal` (`tea-da2isimlm39c739m4ofg`, the QA account), 22:57–23:13Z, pin `19fdb46f1758`: `bex-api` and `bex-ssh-gateway` ran `bex-operator@sha256:7a7051cf…` and every session and builder Pod `bex-agent-sandbox@sha256:de602e4d…`, the digests that pin set (read-only `kubectl`, context `hetzner-prod`). The calls used a QA key minted for the workspace, revoked afterwards. Every session ran `claude` on `deepseek/deepseek-v4.1-flash-1017`, template `agent`, `maxSpendUSD` 1, on the workspace's default `registries` tier, with the task "run these commands and quote them, then sleep".

- **Fixtures.** Three public repositories under the `puncsky` installation, each (disposable): `puncsky/qa-20261122-m544-pyfile` (a root `.python-version` of `3.13`), `puncsky/qa-20261122-m544-pydefault` (no `.python-version`; a `pyproject.toml` with `requires-python = ">=3.12"` and a `uv.toml` with `python-preference = "only-managed"`) and `puncsky/qa-20261122-m544-py27` (`.python-version` `2.7.18`). Each environment had one setup step that printed `python3 --version`, `readline.backend`, `command -v python3` and `/usr/bin/python3 --version` (and, for `pydefault`, any `UV_PYTHON*` variable and `uv run python`).
- **Fresh and snapshot-booted.** A _phase-1_ session was dispatched before its environment had a successful build (`environmentBuildId: null`), so its setup step ran in its own turn. A _snapshot_ session booted from the build (`environmentBuildId` set) and ran no setup step.
- **Web Shell.** An agent session has no browser web Shell (w10/027). An interactive `ssh <session>@ssh.bex.co` on a real local pty (120×24) stood in for it, the web Shell's own `/bin/sh` tty exec: stdin and stdout were ttys, `$-` was `smi`, uid 10001, `ENV=/etc/profile.d/bex-agent-tools.sh`.

| DoD bullet | Phase 1 | Snapshot-booted | Verdict |
| --- | --- | --- | --- |
| 1. `.python-version` `3.13` | `ags-dc1n6m1fdpsc73e0hi90` | `ags-dc1n7spfdpsc73e0higg` (build `arb-dc1n6lt3vdpc73b0i3r0`) | holds |
| 2. `PYTHON_VERSION=3.11.9` | — | `ags-dc1n8q53vdpc73b0i41g` | **fails on `registries`**: fixed by t012 |
| 3. Neither: the dated default | `ags-dc1nbvt3vdpc73b0i4e0` | `ags-dc1ncv53vdpc73b0i4m0` (build `arb-dc1nbvhfdpsc73e0hjlg`) | holds |
| 4. `2.7.18` refused by name | `ags-dc1nbbpfdpsc73e0hjig` (`.python-version`) | `ags-dc1naqhfdpsc73e0hjd0` (secret) | holds |
| 5. uv `only-managed` | `ags-dc1nbvt3vdpc73b0i4e0` | `ags-dc1ncv53vdpc73b0i4m0` | holds |
| 6. The transcript step | every session | every session | holds |

1. **`.python-version` `3.13`.** The phase-1 session's first step was `setup: Python` with `==> Using Python 3.13.18 (from .python-version)`. Its setup step printed `Python 3.13.18`, `readline.backend: readline`, `/opt/bex-python/bin/python3` and `/usr/bin/python3: Python 3.11.2`. In the agent's shell, `python3 --version` and `bash -lc "python3 --version"` gave 3.13.18, and `readline.backend` was `readline`; over SSH, the same. The build (22:57:32–22:59:40Z, `success`, 2.38 GB) recorded the steps `Python` (the same line) and `python-check` (the same output). The session booted from it gave the same in the agent's shell and over SSH.
2. **`PYTHON_VERSION=3.11.9`, a repo-scoped secret on `pyfile`.** A session booted from the 3.13 build selected again after checkout, from `PYTHON_VERSION`, but the fetch failed: `==> Using Python 3.11.18 (from PYTHON_VERSION)`, then `==> Could not fetch python:[redacted:PYTHON_VERSION]-bookworm (curl: (6) Could not resolve host: production.cloudfront.docker.com …); using the reviewed Python 3.11 build, 3.11.18`, and `python3 --version` was 3.11.18. The secret's build `arb-dc1n8od3vdpc73b0i40g` (`secret_changed`) recorded the same fallback in its `Python` step.
   - **Cause.** Docker Hub answers a blob read with a `307` to `production.cloudfront.docker.com` (reproduced from this host for the image's top four layers). That is the pull host [Docker's allowlist](https://docs.docker.com/desktop/setup/allow-list/) names. The `registries` catalog named only `production.cloudflare.docker.com`, and t006's fixture registry served blobs without a redirect.
   - **The fetch itself works.** With the tier set to `full` for one session, then set back to `registries`, `ags-dc1na2pfdpsc73e0hj40` booted from that build and fetched the patch through the egress proxy. Its step read `==> Using Python [redacted:PYTHON_VERSION] (from PYTHON_VERSION)`: the version equals the secret's value, so the transcript redacts it (ADR098 D6). Over SSH, `python3 --version` was `Python 3.11.9`, `/opt/bex-python/bin/python3` linked to `/opt/bex-python/3.11.9/bin/python3.11`, `python3 -m pip --version` ran from `/opt/bex-python/3.11.9/…`, and `readline` was GNU readline.
   - **Fix.** t012 (`ca86145d9f`) adds the host to the catalog. The bullet is replayed once the pin carries it.
3. **Neither.** On `pydefault` the step read `==> Using Python 3.14.10 (default)`. `python3 --version` was 3.14.10 and `readline.backend` was `readline` in the setup step, the agent's shell and SSH, in both sessions, and in the build's steps.
4. **`2.7.18`.** On `py27`, a session with no environment failed within 10 s with `AGENT_SESSION_SETUP_FAILED`: `.python-version "2.7.18" is not a supported Python version; supported Python lines are 3.7, 3.8, 3.9, 3.10, 3.11, 3.12, 3.13, 3.14, 3.15`. The `setup: Python` step showed the error and no agent turn ran. On `pyfile` with the secret set to `2.7.18`, a session booted from build `arb-dc1n8od3vdpc73b0i40g` failed with the same code, naming `PYTHON_VERSION=[redacted:PYTHON_VERSION]` (the secret's value, ADR098 D6). That secret's own build failed `setup_failed` with the same reason. The listed lines include 3.7, which a session refuses with its own message. That is the native build's shared wording, a follow-up and not a gap in the bullet.
5. **uv `only-managed`.** No `UV_PYTHON*` variable was set. `uv run python` downloaded `cpython-3.15.2-linux-x86_64-gnu` (36.6 MiB) into `/home/bex/.local/share/uv/python/` and ran it (`readline.backend` `editline`, python-build-standalone's), in the setup step, the build and both sessions. `python3` stayed 3.14.10 and `/usr/bin/python3` Debian's 3.11.2.
6. **The transcript step.** In every session, the first part after `start` was the `setup: Python` tool call with the line above, before the setup steps and the agent. Each build recorded it as its first step, `Python`.

**Cleanup.** The eight sessions were deleted (`404`), both repository environments deleted (`[]`), the secret deleted (`[]`), and the tier left on `registries`. The three repositories were deleted (`404`), along with the SSH key registered for the run. `tea-da2isimlm39c739m4ofg-sandbox` has no Pod left. The QA key was revoked and the QA sign-in logged out. Model spend for the run was about $0.09.

## Parked again (2026-11-22)

What is left is DoD bullet 2 on the `registries` tier, once the platform pin carries `ca86145d9f` (t012). Every other bullet held on pin `19fdb46f1758` (§ Live check above).

1. **Fixture.** As the QA user in `tian-personal`, create a public (disposable) repository with a root `.python-version` of `3.13`. Set the repo-scoped secret `PYTHON_VERSION=3.11.9` first, while no environment exists, so it queues no build. Then save an environment whose setup step prints `python3 --version` and `python3 -m pip --version`.
2. **Phase 1.** Start a session at once, before that build succeeds.
3. **Snapshot.** After the build succeeds, start a second session, which boots from it.
4. **Check.** Each session's `setup: Python` step, and the build's `Python` step, read `==> Using Python [redacted:PYTHON_VERSION] (from PYTHON_VERSION)` with no `Could not fetch` line. Over SSH, `python3 --version` is `Python 3.11.9` and `python3 -m pip --version` names `/opt/bex-python/3.11.9`.
5. **Close.** Delete the sessions, the environment, the secret and the repository, then close with t011.

No user decision is needed.

## Unblocked (2026-11-23)

- **Gate cleared (verified evidence).** The newest pin on `origin/main` is `e6c65f5c62` (`chore(deploy): pin platform images to 1a5a8e872398`), and `git merge-base --is-ancestor ca86145d9f 1a5a8e872398` holds. Production `bex-api` and `bex-ssh-gateway` Pods run `bex-operator@sha256:65ef4f0d90b1…` and `BEX_AGENT_SESSION_IMAGE` is `bex-agent-sandbox@sha256:e374484c04d1…`, matching `e6c65f5c62`. No gate remains and no user decision is needed.
- **Next.** Replay DoD bullet 2 on the `registries` tier (a phase-1 session and a snapshot-booted session with `PYTHON_VERSION=3.11.9` overriding `.python-version` `3.13`), clean up the fixtures, and close with t011.

## Live check (t011, production, 2026-11-23 — DoD bullet 2 replay)

Workspace `tian-personal` (`tea-da2isimlm39c739m4ofg`, the QA account), 06:42–07:03Z, pin `1a5a8e872398` (`e6c65f5c62`, carrying `ca86145d9f` / t012): `bex-api` and `bex-ssh-gateway` ran `bex-operator@sha256:65ef4f0d90b1…` and the phase-1 sandbox Pod ran `bex-agent-sandbox@sha256:e374484c04d1…`, matching that pin (read-only `kubectl`, `hetzner-prod`). The workspace stayed on its default `registries` tier throughout (`networkTier: REGISTRIES`). Every session ran `claude` on `deepseek/deepseek-v4.1-flash-1017`, template `agent`, `maxSpendUSD` 1.

- **Fixture.** Public repository `puncsky/qa-20261123-m544-pypatch` (disposable) with a root `.python-version` of `3.13`. Repo-scoped secret `PYTHON_VERSION=3.11.9` was set first while no repository environment existed, then an environment was saved with a `python-check` setup step printing `python3 --version`, `python3 -m pip --version`, GNU readline check (Python 3.11 predates `readline.backend`), `command -v python3` and `readlink -f "$(command -v python3)"`.
- **Repository environment build (`arb-dc1u1g8oodks73aud9s0`).** Ran 06:44:26–06:46:19Z, `status: success` (2.41 GB). Step `Python` (3.35 s, exit 0) read `==> Using Python [redacted:PYTHON_VERSION] (from PYTHON_VERSION)` with no `Could not fetch` line. Step `python-check` (1.40 s, exit 0) printed `Python [redacted:PYTHON_VERSION]`, `pip 24.0 from /opt/bex-python/[redacted:PYTHON_VERSION]/lib/python3.11/site-packages/pip (python 3.11)`, `readline: readline`, `/opt/bex-python/bin/python3` and `/opt/bex-python/[redacted:PYTHON_VERSION]/bin/python3.11`.
- **Phase-1 sessions (`environmentBuildId: null`).**
  - `ags-dc1u1gfqbg1c739j1ff0` (started 06:44:17Z, while `arb-dc1u1g8oodks73aud9s0` was still `queued`): `setup: Python` read `==> Using Python [redacted:PYTHON_VERSION] (from PYTHON_VERSION)` with no `Could not fetch` line; `setup: python-check` and the agent's shell both printed `Python [redacted:PYTHON_VERSION]`, `pip 24.0 from /opt/bex-python/[redacted:PYTHON_VERSION]/lib/python3.11/site-packages/pip (python 3.11)` and `/opt/bex-python/[redacted:PYTHON_VERSION]/bin/python3.11` (`readline: readline` in setup).
  - `ags-dc1u8sooodks73audavg` (pure phase-1 after deleting the repository environment, `environmentBuildId: null`): `setup: Python` read the same line with no `Could not fetch` fallback; the agent's shell printed the same versions and path; over interactive `ssh ags-dc1u8sooodks73audavg@ssh.bex.co` on a 120×24 local pty (`flags=smi`, `uid=10001`, `ENV=/etc/profile.d/bex-agent-tools.sh`, after `setup: Python` finished), `python3 --version` was `Python 3.11.9`, `python3 -m pip --version` was `pip 24.0 from /opt/bex-python/3.11.9/lib/python3.11/site-packages/pip (python 3.11)`, GNU `readline` was active, `/opt/bex-python/bin/python3` resolved to `/opt/bex-python/3.11.9/bin/python3.11`, and `/usr/bin/python3` stayed Debian's `Python 3.11.2`.
- **Snapshot-booted session (`ags-dc1u2fgoodks73auda9g`, `environmentBuildId: arb-dc1u1g8oodks73aud9s0`).** Booted from the snapshot image (`arb-dc1u1g8oodks73aud9s0@sha256:6f4359aa…`, no `setup: python-check` re-run). `setup: Python` read `==> Using Python [redacted:PYTHON_VERSION] (from PYTHON_VERSION)` with no `Could not fetch` line. The agent's shell and interactive 120×24 SSH PTY both confirmed `Python 3.11.9`, `pip 24.0 from /opt/bex-python/3.11.9/lib/python3.11/site-packages/pip (python 3.11)`, GNU `readline`, `/opt/bex-python/bin/python3` → `/opt/bex-python/3.11.9/bin/python3.11`, and `/usr/bin/python3` `Python 3.11.2`.
- **Verdict.** DoD bullet 2 holds on the default `registries` tier in both phase-1 and snapshot-booted sessions. Combined with bullets 1 and 3–6 on pin `19fdb46f1758`, the entire definition of done holds.
- **Cleanup.** All four sessions deleted (`agentSessions: []`), repository environment deleted (`[]`), `PYTHON_VERSION` secret deleted (`[]`), ephemeral SSH key `ssk-dc1tvg0oodks73aud9l0` deleted, workspace left on `REGISTRIES`, disposable repository `puncsky/qa-20261123-m544-pypatch` deleted, `tea-da2isimlm39c739m4ofg-sandbox` has no Pod left, and both QA Kratos sessions logged out. Model spend was ~$0.02.

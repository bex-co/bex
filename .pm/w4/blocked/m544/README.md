# w4 · m544 — An agent session runs the Python its repo asks for, chosen by Render's rules, from the official builds with GNU readline

**Worker:** worker4 **Goal:** a cloud agent session picks its Python the way a Render native build does (`PYTHON_VERSION` → `.python-version` → Render's dated default), from the same table the hosted-app build uses, and runs that version from the official Docker Python builds (GNU readline) in every shell and setup step. **Status:** blocked (t001–t010 done; t011 Closeout waits for production to carry `344463497`)

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
| t011 | Closeout                                                                          | 15m | t009, t010       |

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
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)) run a repository's own toolchain, and the hosted-app build already follows Render's version rules ([ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md)); one table keeps the two from drifting.
- **Expected outcome:** today every session gets the baked 3.12.16 (or uv's download) whatever the repository asks for (w10/m513); after this, an agent runs, tests and debugs on the Python the repository's deploy would get on Render.
- **Why now:** agent automations run on Python repositories every few hours, and a session on the wrong line produces work that fails on the line the repository actually uses. The resolver is still operator-internal (`lego/operator/internal/build/nativeversion.go`), so the move to `lego/types` must come before any session-side caller.
- **Render parity:** included (t008). Selection follows Render's documented rules; the agent-side behavior (a visible step before setup, the PATH reaching setup steps and terminals) is checked against Devin, the reference for agent surfaces (ADR047).

## Parked 2026-11-21

t001–t010 shipped in `344463497`. What is left is t011: replaying the definition of done on production agent sessions, fresh and booted from a snapshot, once the platform pin carries that commit (the agent image is built and its build check, which this host could not run, proves the baked lines on that deploy). Then, in a repository session on the QA workspace: a root `.python-version` `3.13` gives `python3 --version` 3.13.18 and `readline.backend == "readline"` in the agent's shell, a setup step and the web Shell; a `PYTHON_VERSION=3.11.9` repo secret gives exactly 3.11.9 (its value shows redacted in the transcript, ADR098 D6); a repository with neither gets 3.14.10; `2.7.18` fails `AGENT_SESSION_SETUP_FAILED` naming it; a uv `python-preference = "only-managed"` repository still downloads uv's build while `/usr/bin/python3` stays 3.11; the transcript's first step reads `==> Using Python <version> (from …)` or `(default)`. Record the evidence here and close with t011.

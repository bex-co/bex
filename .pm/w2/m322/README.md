# w2 · m322 — Idle-suspended sandboxes keep their memory and running processes, as on Render

**Worker:** worker2 **Goal:** when a sandbox idle-suspends (w2/m278), its memory, running processes and `/tmp` survive alongside its filesystem, and the next exec resumes them where they stopped, as Render's runtime snapshot does. A customer's dev server or REPL is still running after the resume. **Status:** todo

## Tasks (in order)

| id   | title | est | depends_on |
| ---- | ----- | --- | ---------- |
| t001 | Feasibility on the production substrate: checkpoint and restore a gVisor sandbox pod | 2h | — |
| t002 | Design record: where memory images live, how resume restores them, failure fallback | 45m | t001 |
| t003 | Idle suspension takes a runtime checkpoint, and resume restores it | 2h | t002 |
| t004 | ADR018/ADR042 record the result; the temporary difference left by w2/m278 closes | 20m | t003 |
| t005 | Production walk: a process started before the suspension is still running after the exec resumes it | 30m | t004 |
| t006 | Render parity | 20m | t005 |
| t007 | Simplify | 15m | t006 |
| t008 | Test coverage | 30m | t007 |
| t009 | Closeout | 10m | t008 |

## Definition of done

- On production, a sandbox runs a background process that holds state in memory (e.g. a counter in a long-running process) and is left idle past its timeout. It goes `suspended`, and `bex sandboxes exec` resumes it. The process is still running with its in-memory state intact, and `/tmp` and the filesystem are unchanged.
- A failed restore falls back to the filesystem-only resume of w2/m278 rather than losing the sandbox, and says so in the sandbox's events.
- ADR018's sandbox idle row no longer lists the memory/process difference, and ADR042 D5 records the mechanism.
- **If t001 shows the substrate cannot checkpoint gVisor pods** on any route that the platform can carry, stop and bring the evidence back to the user. Do not ship a partial mechanism.

## Source + Goal linkage

- **Source:** `/unblock w2` 2026-11-11, user decision D1=A on w2/m278 ("投入做内存快照"). w2/m278 shipped filesystem-preserving idle suspension, and memory/processes/`/tmp` are lost on suspend.
- **Render:** an idle sandbox "resumes when your agent sends its next command, with its files, memory, and running processes intact". The default runtime snapshot "saves memory and running processes" ([Render Sandboxes](https://render.com/sandboxes), [creating & configuring](https://render.com/docs/sandboxes-creating), [snapshots](https://render.com/docs/sandboxes-snapshots), fetched 2026-11-11).
- **bex today:** ADR042 D5. The OpenSandbox k8s BatchSandbox pause commits the rootfs to an OCI image in a confined `opensandbox-snapshot` Job, and resume recreates the pod from it. Memory hibernation was a "watch item" (containerd checkpoint/restore / CRIU; for gVisor, `runsc checkpoint`/`restore`).
- **Goal linkage:** ADR047 cloud coding-agent sessions and ADR008 parity. Agents rely on long-lived processes (dev servers, language servers) between turns.
- **Expected outcome:** suspending an idle sandbox no longer kills a customer's running processes, so idle suspension stops being a reason to set `--idle-timeout 0` (which keeps billing).
- **Why now:** w2/m278 turned idle suspension on by default (900 s), so every customer sandbox now loses its processes after 15 idle minutes. w32/m285 (`--kind runtime` snapshots, Render's default) also waits on this mechanism.
- **Render parity included:** sandbox behavior on REST/GraphQL/MCP/CLI.

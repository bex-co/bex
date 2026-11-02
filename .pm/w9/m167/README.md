# w9 · m167 — `bex edit`: the platform CLI delegates editing to bex desktop

**Worker:** worker9 **Goal:** `bex` means one thing — the platform CLI — and it opens code in bex desktop through `bex edit [path…|ags-id]` plus a strict path-shaped shorthand `bex <path>`, without shadowing any imported Render command. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Locate bex desktop and `bex edit <path…>` exec delegation | 60m | — |
| t002 | Path-shaped shorthand `bex <path>` in the launcher pre-dispatch | 45m | t001 |
| t003 | `bex edit ags-…` opens an agent-session sandbox in bex desktop | 45m | t001 |
| t004 | Installer and docs: no silent clobber of the desktop link, document the contract | 40m | t001, t002 |
| t005 | Render parity | 20m | t002, t003, t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- `bex edit .`, `bex edit a.go:12:3` and `bex edit -n dir` exec the bex desktop editor CLI with the arguments passed through unchanged. The editor is found on PATH as `bex-desktop` or, failing that, inside the installed app bundle (macOS `Bex*.app`, Linux `~/.local/bex*.app/bin/`). With neither present, the command exits nonzero with an install hint. It never execs any binary named `bex`, so there is no self-recursion into a pre-rename desktop link.
- An existing, path-shaped first argument behaves as `bex edit <path>`. Path-shaped means `.` or `..`, a `./ ../ / ~` prefix, or containing `/`, optionally with a `:line[:col]` suffix. Examples: `bex .`, `bex ./docs`, `bex ../x`, `bex ~/p`, `bex src/main.go:42`.
- Everything else keeps today's behavior. `bex docs` still runs the Render `docs` command. `bex servcies` still prints cobra's unknown-command suggestion. Bare `bex` still prints help. A non-existent path-shaped argument errors without creating a file.
- In a non-interactive session (no TTY, or `CI` set), both forms fail fast with a clear message instead of launching a GUI.
- `bex edit ags-<id>` resolves the session's `sshAddress` through bex-api and hands bex desktop the `ssh://` URL. A session without SSH access (no `can_view_sensitive`, or the sandbox is closed) fails with the API's error, not a hang.
- One launch telemetry event is emitted before exec, as `bex code` does. It carries the command path only, never arguments.
- `scripts/install-bex.sh` warns before replacing a `bex` that is a symlink into a Bex desktop bundle. `docs/bex-cli.md` documents `bex edit`, the shorthand rule, and the cross-repo contract with bex-desktop `docs/adrs/ADR002`.
- `cd lego/cli && go test ./...` and `make lint` pass.

## Source + Goal linkage

- **Source:** user discussion 2026-11-01/02.
  - **Problem:** bex desktop (`bex-co/bex-desktop`, a Zed fork) installs its editor launcher as `bex`, into the same places the platform CLI uses (`/usr/local/bin/bex`, `~/.local/bin/bex`). `scripts/install-bex.sh:110` overwrites with `mv -f`. So `bex` meant different programs depending on install order.
  - **Decision:** `bex` is the platform CLI. The desktop launcher becomes `bex-desktop`, shipped separately in the bex-desktop repo. The platform CLI gains `bex edit` and a path-shaped shorthand. `bex open` is not used for this: it stays reserved for "view a service or session in the browser", the `heroku`/`fly`/`netlify open` convention.
  - **Verified:** the pinned Render CLI has no `open`, `edit` or `desktop` command.
- **Goal linkage:**
  - [ADR008](../../../docs/ADR008-vision.md): an AI-native developer surface with one coherent command namespace across bex CLI and bex desktop.
  - [ADR054](../../../docs/ADR054-open-in-zed.md): agent-sandbox editor remoting gets a terminal entry point.
  - Complements [ADR092](../../../docs/ADR092-coding-agent-mcp.md).
- **Expected outcome:**
  - Existing desktop users keep typing `bex .`.
  - Platform users never lose `bex login` or `bex services` to an editor link.
  - `bex edit ags-…` goes from a session id in CI output straight into the sandbox.
- **Why now:** the name collision is live for anyone with both products installed. The bex-desktop rename removes the desktop's `bex` link, so without `bex edit`, `bex .` stops working for desktop users the moment they update.
- **Parity:** included, because this is a tenant-facing CLI surface. The pinned Render CLI is the reference: it has no equivalent command, and the shorthand must not shadow any upstream command. For `ags-` targets the reference is Devin's editor-workspace handoff and the dashboard's Open-in-Zed.

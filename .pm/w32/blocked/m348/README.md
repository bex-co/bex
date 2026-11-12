# w32 · m348 — A sandbox command runs under `bash -c` in the home directory, as on Render

**Worker:** worker32 **Goal:** a command sent to a Render sandbox on bex (CLI `sandboxes exec`, the Render SDK's `exec`, REST, GraphQL or MCP) starts in the sandbox user's home directory and is interpreted by `bash -c`, so the documented "copy a file in, then run it" workflow and bash syntax work unchanged **Status:** blocked (t001–t004, t007, t008 done; t005 waits on the production deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: Render's exec contract, the base image and the correction of w22/001 — **DONE** | 45m | — |
| t002 | The base sandbox image ships `bash` and every tool the file gateway's scripts use — **DONE** | 1.5h | t001 |
| t003 | Exec starts in `$HOME` and runs the command under `bash -c`, falling back to `sh` — **DONE** | 1h | t001 |
| t004 | Blast radius: agent-session sandboxes, suspended rootfs images and the other `/bin/sh` callers — **DONE** | 30m | t003 |
| t005 | Production walk with the release CLI and the isolation matrix | 45m | t002, t003, t004 |
| t006 | Render parity | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

On production, with the released `bex` CLI against a new sandbox in `bex-canary`:

- `bex sandboxes copy ./run.sh <sbx>:run.sh` followed by `bex sandboxes exec <sbx> -- sh run.sh` runs the script (exit 0, its output), and `bex sandboxes exec <sbx> -- pwd` prints the sandbox user's home directory, not `/`.
- `bex sandboxes exec <sbx> -- bash -c 'arr=(a b); echo ${arr[1]}; echo {1..3}'` prints `b` and `1 2 3` (exit 0). A command string sent by the Render SDK (or REST `exec`) is interpreted by bash: the same array and brace syntax works without a `bash -c` prefix.
- A sandbox whose root filesystem has no `bash` (one created or committed before this change and resumed from it) still runs commands, through `/bin/sh` in `$HOME`.
- REST, the connect-token run path, GraphQL and MCP `sandbox_exec` behave the same way. Agent-session sandboxes keep the behavior t004 records.
- `docs/ADR104-sandbox-render-parity.md` D2, the ADR018 sandbox exec row and `docs/render-artifacts/ea-sandbox.md` say `bash -c` in the home directory, citing the SDK docstring. The `w22/001` premise correction is itself corrected.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep 865a, 2026-11-12, in `bex-canary` (`tea-daif693dqjvc73e7as3g`) with release `bex` v0.5.0 (Render CLI v2.35.0 pin) against `https://api.bex.co/v1/`, human device login. The evidence is below.
- **Goal linkage:** pillar 5 sandboxes and [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). ADR104 D9 makes the Render SDK part of the contract, and its `exec` is the main programmatic path.
- **Expected outcome:** SDK and CLI scripts written for Render sandboxes run unchanged. A relative upload can be executed by a relative path, and bash syntax works.
- **Why now:** the bug breaks the most basic sandbox workflow, including the CLI help's own example (`copy ./main.py sbx:main.py`, then `exec sbx -- python script.py`). w22/001 closed the `bash -c` question the wrong way on 2026-11-10, so nothing else on the board will catch it. The w32/m284 SDK walk would hit it on the first bash-syntax command.
- **Render parity:** included. The change alters exec behavior on REST, GraphQL and MCP; the dashboard has no sandbox exec surface.

## Evidence (sweep 865a, 2026-11-12 ~07:30–07:40 UTC)

**Render's contract:**

- Render Python SDK `render==1.8.0`, `render/experimental/sandbox/client.py:357-358`: `exec` docstring "command is passed to ``bash -c`` in the sandbox." This was read from the PyPI wheel, sha256 `80800015c17288156ddafced46ec393f134e138b5ade402a47f7f641f58906c1`.
- render.com/docs/sandboxes-executing (fetched 2026-11-12): "Each command starts in a new shell in your sandbox's home directory." Working-directory changes are not kept between commands.
- render.com/docs/sandboxes-sdk-python (fetched 2026-11-12): relative remote paths "resolve under `/home/render`". bex resolves them under `$HOME` = `/root` (w32/m299).
- w22/001 read the `/docs/sandboxes-executing` advice "Use `sh -c` to interpret operators such as `&&`, pipes, and redirection" as the server's shell. That sentence is CLI advice: the CLI shell-quotes its argv into one command string. The SDK docstring states the server-side shell.

**bex on production** (sandbox `sbx-dbqmm42ibtfs738vknn0`, created with `--wait --timeout 900 --idle-timeout 300 --network-policy deny-all`, since terminated):

```text
$ bex sandboxes copy ./up/run.sh sbx-…:run.sh
Uploaded ./up/run.sh to sbx-…:run.sh                                  (exit 0)
$ bex sandboxes exec sbx-… -- sh run.sh
sh: can't open 'run.sh': No such file or directory                    (stderr, exit 2)
$ bex sandboxes exec sbx-… -- sh -c 'pwd; echo HOME=$HOME; ls -la ~ | head -5'
/
HOME=/root
… -rw------- 1 root root 17 … run.sh                                  (exit 0)
$ bex sandboxes exec sbx-… -- bash -c 'echo bash=$BASH_VERSION'
/bin/sh: bash: not found                                              (stderr, exit 127)
$ bex sandboxes exec sbx-… -- sh -c 'echo {1..3}'
{1..3}                                                                (exit 0; bash prints 1 2 3)
$ bex sandboxes exec sbx-… -- sh -c 'arr=(a b); echo ${arr[1]}'
sh: syntax error: unexpected "("                                      (stderr, exit 2)
$ bex sandboxes exec sbx-… -- sh -c 'cat /etc/os-release | head -2; ls -l /bin/sh'
NAME="Alpine Linux" … /bin/sh -> /bin/busybox
```

The same sandbox's exit codes, stderr, the env marker (`--env-var`), a directory upload, a 3 MB upload, and the download round trip (sha256 equal) all behaved correctly.

**Root cause (traced in code at `02c656c19`):**

- `lego/backend/internal/sandbox/exec.go:120` `dialGateway` wraps every caller command as `[]string{"/bin/sh", "-c", req.Command}` with no working directory. REST exec, the connect-token run path (`connect.go:276` → `StreamExec`), GraphQL and MCP all go through it. The pod's working directory is the image's (`/` for Alpine).
- `lego/backend/cmd/api/config.go:560` sets the default `BEX_SANDBOX_IMAGE` to `docker.io/library/alpine:3@sha256:28bd5fe8…` (the `base` template, `cmd/api/main.go:96-99`, entrypoint `sleep infinity`). That image has busybox `ash` and no `bash`, `python3`, `git` or `curl`.
- `exec_test.go:631` `TestExecHandsTheCommandStringToShAsOneUntouchedArgument` pins `/bin/sh` and must change with the contract.

**Unverified:**

- Render's default image contents beyond bash: python, git and curl are not stated in Render's docs (render.com/docs/sandboxes*, the launch post; fetched 2026-11-12). t001 records what bex ships and why.
- The user name and whether Render's home is `/home/render` for every command. t001 decides whether bex moves `HOME` from `/root`.
- Whether Render creates missing parent directories on upload. Not stated; bex answers `404 … destination parent does not exist` for `rel/f.txt` when `rel/` is missing. Not filed.

## Parked 2026-11-12 (`/loopx w32`): t005 waits on the production deploy

- **Shipped:** `60879160d` (exec wrapper, `bash:5.3` base image, `/bin/bash` link, isolation-matrix check); with it, on the same train, w32/015 (`c35fa52ab`, list newest first), w32/016 (`e607058d1`, one region), w32/017 (`c8a8c0bcd`, lifetime expiry), w32/018 (`717022990`, upload mtimes and modes) and w32/019 (home `/home/render`).
- **Gate:** production deploys are batched (`deploy.yml` deploys a push or the hourly schedule once the pin is 6 h old); the pin was `33e1b1e6117c` at 18:39 UTC. No user decision is needed: once `git merge-base --is-ancestor <shipped sha> <pinned sha>` holds, a worker runs the walk.
- **Sign-in works:** `bash scripts/qa-login.sh` established a session on production at ~19:50 UTC (and was logged out), so the CLI device login can be approved through `--login-challenge`.
- **The walk (t005, then t006 and t009):** in `bex-canary` with the released CLI:
  1. `bex sandboxes create --wait --timeout 900 -o json`; `copy ./run.sh <sbx>:run.sh` then `exec <sbx> -- sh run.sh` → `from-script`; `exec <sbx> -- pwd` → `/home/render`; `exec <sbx> -- bash -c 'arr=(a b); echo ${arr[1]}; echo {1..3}'` → `b`, `1 2 3`.
  2. w32/015: `bex sandboxes list --all -o json` lists the newest `createdAt` first.
  3. w32/016: the sandbox's `region` equals its group's in `bex sandbox-groups list -o json` (`frankfurt`).
  4. w32/017: `create --wait --timeout 60`, then at +90 s `exec <id> -- true` → 409 "was terminated at <createdAt+60s>".
  5. w32/018: `copy ./m <sbx>:/tmp/m` then `stat -c '%a %Y'` shows the local file mtimes and `/tmp/m` is not 700; a single uploaded file is 644.
  6. Run `isolation-matrix.yml` and record its run id here.

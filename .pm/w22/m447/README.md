# w22 · m447 — The default sandbox image runs Render's three official agent templates

**Worker:** worker22 **Goal:** a sandbox created with no image runs the executor install scripts of Render's three first-party agent templates (Claude Managed Agents, Cursor Cloud Agents, OpenAI Agents API) unchanged, under each template's own network allow-list, and the installed executor starts. ADR104's open question 5 (what the default image must contain) is answered from that evidence. **Status:** todo (t001, t002 done)

## Tasks (in order)

| id   | title                                                                                                                                         | est | depends_on       |
| ---- | --------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Read the three templates' install scripts and launch policies, list everything they assume of the image, and run each in today's image — **DONE** | 50m | —                |
| t002 | Design record in ADR104 D2: the new base image, what it carries and why, and open question 5 answered with the limits of the evidence — **DONE** | 30m | t001             |
| t003 | Move the `base` template to the new image, digest-pinned, with its entrypoint and `/bin/bash` kept working                                    | 60m | t002             |
| t004 | Re-prove what depended on the old image: name resolution under an allow-list, the file gateway's scripts, and the isolation matrix's probes   | 75m | t003             |
| t005 | Sandboxes created before the change: they keep their image across suspend and resume, stated in the docs and pinned by a test                 | 30m | t003             |
| t006 | Production check: the three install scripts exit 0 under their allow-lists in a default sandbox and each executor prints its version          | 60m | t004             |
| t007 | Render parity                                                                                                                                 | 20m | t004, t005       |
| t008 | Simplify                                                                                                                                      | 20m | t007             |
| t009 | Test coverage                                                                                                                                 | 30m | t007             |
| t010 | Closeout                                                                                                                                      | 10m | t006, t008, t009 |

## Definition of done

- On production, in a sandbox created with no image and with each template's own allow-list as its network policy, the template's install script exits 0 and the executor runs: Cursor's `agent --version`, the `ant` CLI's version command, and `codex --version` (or what each template's script itself runs to verify the install).
- The weekly isolation matrix is green on the new image, including the allow-list name-resolution probes and the file upload and download legs.
- `bash -c` exec in `$HOME`, `#!/bin/bash` scripts and the tools users had before (`wget`, `tar`, `sha256sum` and the busybox applets ADR104 names) still work in a new sandbox.
- A sandbox created before the change still resumes and runs commands.
- ADR104 D2 names the image and its digest, lists what it carries, and open question 5 is closed with the evidence and what stays unverified. ADR018's sandbox rows agree.
- No model or agent-platform account was needed: the check ends when the executor is installed and starts.

## Template requirements (t001, measured 2026-11-23)

The three templates, cloned at `render-examples/cursor-cloud-agent-sandbox` `f7bdb349875`, `render-examples/claude-managed-agents-sandbox` `b4b182bd0f7` and `render-examples/openai-agents-api-sandbox` `2f391c75e73`. Each creates a sandbox with no image and no plan, uploads `scripts/install-executor.sh` to `/tmp/install-executor.sh` and runs it with its own command: Cursor `mkdir -p /workspace/output && chmod 0700 /tmp/install-executor.sh && /tmp/install-executor.sh`, Claude `bash /tmp/install-executor.sh`, OpenAI `chmod 0700 /tmp/install-executor.sh && /bin/bash /tmp/install-executor.sh`. All three run as root (they write `/workspace`, npm's global prefix and, for Claude, may mount devpts) and need no account to install.

| Requirement | Needed by | Today's image (`bash:5.3`, Alpine 3.24, musl, busybox) |
| --- | --- | --- |
| `#!/usr/bin/env bash`, `/bin/bash`, `bash` on PATH | all three | present (`/bin/bash` linked by the entrypoint) |
| `curl` (download; Cursor's readiness probe `curl -fsS http://127.0.0.1:8080/readyz`) | Cursor, Claude | **missing**: first failure of both, `curl: command not found`, exit 127 |
| GNU `sha256sum --check -` | Cursor, Claude | **busybox has no `--check`**: second failure of both, exit 1 |
| glibc ≥ 2.28 (x64) / ≥ 2.34 (arm64) and libstdc++ (`GLIBCXX_3.4.22`) | Cursor: its agent bundles a dynamically linked Node (`/lib64/ld-linux-x86-64.so.2`, `/lib/ld-linux-aarch64.so.1`) and glibc `.node` addons | **musl**: third failure, `…/node: cannot execute: required file not found`, exit 127 |
| Node.js and `npm` on PATH (`npm view`, `npm install --global`) | OpenAI | **missing**: first failure, `npm: command not found`, exit 127 |
| GNU `find -P … -printf '%P\0'` (collecting `/workspace/output`) | Cursor, OpenAI | **busybox rejects `-printf`** |
| `find -print0 \| base64 -w0`, `tar -xzf` (`--strip-components`), `mktemp`, `uname`, `ln`, `install -d -m`, `grep`, `mount -t devpts` (only when `/dev/pts` is not mounted) | Claude; Cursor | present (busybox) |
| linux `x64`/`amd64` or `arm64` | all three (both are pinned with checksums) | either |

The executors: Cursor agent `2026.10.19-6bbf2c4` (Node bundle, glibc, both architectures), `ant` 2.1.0 (static Go), `@openai/codex` 0.163.0 (static musl, x64 `static-pie`). Downloaded archives matched every checksum the scripts pin.

Launch policies (network policy `allow-list`, every rule `https`; no plan, so `1c-2g`):

| Template | Install hosts | Runtime hosts | Env and timeout |
| --- | --- | --- | --- |
| Cursor | `downloads.cursor.com` | `api2.cursor.sh`, `api2direct.cursor.sh`, `agent-stores.s3.us-east-1.amazonaws.com`, `cloud-agent-artifacts.s3.us-east-1.amazonaws.com`, `cursor.com` | `HOME=/tmp/cursor-home`, its own `PATH`, `CURSOR_DISABLE_AUTO_UPDATE=1`, worker id and pool; the caller's timeout |
| Claude | `github.com`, `release-assets.githubusercontent.com`, `objects.githubusercontent.com` | `api.anthropic.com` | `ANTHROPIC_ENVIRONMENT_ID`, `ANTHROPIC_ENVIRONMENT_KEY`; `min(86400, max(300, run + 120))` s |
| OpenAI | `registry.npmjs.org` | `api.openai.com`, `codex-cloud-environments.chatgpt.com` | `CODEX_API_KEY`; `min(86400, run + 120)` s |

**Adding `curl` to the Alpine image is not enough.** Run in a container of the pinned `bash:5.3` digest with bex's entrypoint, `HOME` and exec wrapper (linux/arm64, 2026-11-23), working around each failure by hand: with `curl` added, Cursor and Claude stop at `sha256sum --check`; with GNU coreutils added as well, Claude installs and `ant --version` prints `ant version 2.1.0`, and Cursor's agent cannot start on musl; with Alpine's `nodejs` and `npm`, OpenAI installs and prints `codex-cli 0.163.0`. So the image needs glibc, GNU coreutils and findutils, and Node with npm.

The same three runs on the new image (`deploy/sandbox-base/`, built locally from the same base digest) exit 0 on linux/arm64 and on linux/amd64 (emulated): `Cursor Agent CLI: 2026.10.19-6bbf2c4`, `ant version 2.1.0`, `installed codex-cli 0.163.0`. How to reproduce: clone the three repositories outside this one, start the image with `/bin/sh -c` and the base entrypoint (`lego/backend/cmd/api/main.go`), copy the script to `/tmp/install-executor.sh` and run the template's command above through the exec wrapper (`lego/backend/internal/sandbox/exec.go`). This machine's container network could not reach Cloudflare or Fastly hosts, so the containers reached them through a forward proxy on the host; the scripts ran unchanged.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w22` (2026-11-16), item 2, filed by `/pm 2 to w22 as well`.
  - **Render** (read 2026-11-16): three first-party integration pages under Sandboxes, [sandboxes-claude-managed-agents](https://render.com/docs/sandboxes-claude-managed-agents), [sandboxes-cursor-cloud-agents](https://render.com/docs/sandboxes-cursor-cloud-agents) and [sandboxes-openai-agents-api](https://render.com/docs/sandboxes-openai-agents-api). Each is a template that creates a sandbox with no custom image, uploads and runs an install script, starts an executor and collects `/workspace/output`.
    - Cursor template (`github.com/render-examples/cursor-cloud-agent-sandbox`, `scripts/install-executor.sh`): `#!/usr/bin/env bash`; calls `uname`, `mktemp`, `curl`, `sha256sum`, `tar`, `ln`; downloads a prebuilt `linux` `x64` or `arm64` archive from `downloads.cursor.com`; no `sudo`; installs under `$HOME/.local`.
    - Claude template: installs "a pinned, checksum-verified `ant` CLI" from GitHub release hosts; allow-list `api.anthropic.com` plus those hosts.
    - OpenAI template: runs an install script and `codex exec-server`; allow-list `api.openai.com`, `codex-cloud-environments.chatgpt.com` and `registry.npmjs.org`.
    - The Claude and OpenAI scripts were read only through their documentation pages; t001 reads the scripts themselves.
  - **bex today:** the default image is `docker.io/library/bash:5.3` by digest (`lego/backend/cmd/api/config.go` ~line 607, `BEX_SANDBOX_IMAGE`): Alpine 3.24, musl, busybox, bash at `/usr/local/bin/bash` linked to `/bin/bash` by the entrypoint (ADR104 § D2 "Image", w32/m348). It has no `curl`, so the Cursor script fails at its download. **An inference, not measured:** the prebuilt executors are likely linked against glibc and would not start on musl even with `curl` added; t001 measures it.
  - ADR104 open question 5 ("What does Render's default sandbox image ship beyond bash?") has no owner. ADR104 rejected a Debian base in w32/m348 "because it drops `wget` and busybox, which the isolation matrix and users relied on".
- **Goal linkage:** ADR008 § Render parity first; ADR104 D1 (the parity target is Render's documented contract) and open question 5.
- **Expected outcome:** a user who points one of Render's official templates at bex gets a working executor without building their own image.
- **Why now:** the three pages are the use Render leads with for the beta. Every sandbox created on the old image keeps it until it is recreated, so the population that needs the fallback grows each day the change waits.
- **Open recommendations, not user decisions** (the tasks implement them under the parity rule; t002 records them in ADR104 as recommendations):
  - Move to a glibc, Debian-family slim base carrying `bash`, `curl`, `wget`, `ca-certificates`, `tar`, `git` and busybox. This amends ADR104's w32/m348 rejection of Debian: its stated reason is met by installing `wget` and busybox, and the parity rule says to amend an earlier decision that conflicts with parity, not to keep the divergence.
  - If t001 shows the executors run on musl once `curl` is present, the smaller change (add `curl` to the Alpine image) is the parity-sufficient one and t002 chooses it.
  - **What stays unverified:** Render's actual default image. bex has no Render account, so the three templates are the only first-party evidence; the image is sized to them, and the contents list is bex's choice until Render documents its own.
- **Cost and abuse note:** a larger base image is pulled on every sandbox start. t003 records the image size and the cold-start pull time before and after; ADR104 D11's bounds are unchanged.
- **Render parity included:** the default image is customer-facing behavior of sandbox create on REST, GraphQL and MCP, so t007 checks the three surfaces create the same image and compares with Render's pages.

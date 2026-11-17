# w22 · m447 — The default sandbox image runs Render's three official agent templates

**Worker:** worker22 **Goal:** a sandbox created with no image runs the executor install scripts of Render's three first-party agent templates (Claude Managed Agents, Cursor Cloud Agents, OpenAI Agents API) unchanged, under each template's own network allow-list, and the installed executor starts. ADR104's open question 5 (what the default image must contain) is answered from that evidence. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                         | est | depends_on       |
| ---- | --------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Read the three templates' install scripts and launch policies, list everything they assume of the image, and run each in today's image        | 50m | —                |
| t002 | Design record in ADR104 D2: the new base image, what it carries and why, and open question 5 answered with the limits of the evidence         | 30m | t001             |
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

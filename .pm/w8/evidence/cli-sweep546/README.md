# CLI sweep 546 — root-directory traversal refuses at create

**Verdict: no bug filed.** Refusal-only sweep (service count stayed at the 9-row baseline; nothing was created). Same isolated session, `bex-canary`, HEAD-built CLI, 2026-10-19T11:05Z.

| probe | observed result |
| --- | --- |
| `--root-directory ../etc` | Exit `1`, `400: rootDirectory must be a relative path with no '..' components`. |
| `--root-directory examples/../../etc` | Same named `400` — embedded traversal caught, not just a leading `..`. |
| `--root-directory /abs/path` | Same `400` (message's "relative path" clause covers it). |
| `--dockerfile-path …` | Not a flag at this pin: client-side `unknown flag` with usage; no request sent, nothing to traverse through that spelling. |

Companion to sweep 545: shape-invalid root paths never reach a build, while shape-valid-but-absent paths fail by name in the clone step.

- Deploy lag means deployed revisions can precede local `main` (pin still `8e57d4eaa121`; w8/129, /130, /131 fixes are all on `main` ahead of it).

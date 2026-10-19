# CLI sweep 507 — `blueprints validate` file-argument dialects

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build (rebuilt at `37dffc290`; tree since at `0952611b8`, no `lego/cli` delta in the journeys), 2026-10-19T09:08Z. No resources created — validation only.

| probe | observed result |
| --- | --- |
| Missing file / directory as the argument | Exit `1`, named client errors (`file not found: …`, `failed to read file: … is a directory`) before any request. |
| Empty file | Exit `1`, structured `valid:false` — "Blueprint must contain one YAML document". |
| Broken YAML | Exit `1`, `valid:false` with `line`/`column` location. |
| `services: []` in a `.txt` file | Exit `1`, "must define at least one service, database, env group, or project environment resource" — validation is content-based, extension-agnostic. |
| `examples/hello-go/render.yaml` | Exit `0`, `valid:true` with a plan (`totalActions: 1`). |
| No argument in a directory without `render.yaml` | Exit `1`, `file not found: <cwd>/render.yaml` — the default path is named. |
| No argument with only `bex.yml` present | Same `render.yaml` not-found. Explicit `validate ./bex.yml` validates fine (exit `0`). Not filed: `render.yaml` is canonical and `bex.yml` a deprecated alias (ADR049 D1); the default path lives in the upstream command's `RunE`, which bex deliberately does not fork or rewrite (`docs/bex-cli.md` — residuals stand until an explicit fork/extension decision), and the server-side discovery fallback governs repo/Git discovery, not this local default. Recorded here so the dialect is a decision-visible fact, not rediscovered as a surprise. |

## Safety and unrun checks

- Nothing created or mutated; validation is read-only against the workspace.
- Unrun: oversized-manifest 413 dialect over the CLI (server limit recorded in ADR018/w8-m48), multi-document YAML, `-w` workspace flag with a foreign/synthetic workspace ID.
- Deploy lag means deployed revisions can precede local `main`.

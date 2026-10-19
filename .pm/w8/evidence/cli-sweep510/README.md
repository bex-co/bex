# CLI sweep 510 — shell completion generation and `__complete` protocol

**Verdict: no bug filed.** Same isolated session, HEAD build `0952611b8`, 2026-10-19T09:11Z. Local command surface only; nothing created.

| probe | observed result |
| --- | --- |
| `completion bash\|zsh\|fish\|powershell` | All exit `0` (7.6–16 KB); zero `render` hits in any generated script; `bash -n` and `zsh -n` parse clean. |
| `__complete services ""`, `__complete pg ""` | Exit `0`, `ShellCompDirectiveNoFileComp`; descriptions are Bex-branded through the serialization path (`Create a new Bex Postgres database`, …) — the exact seam the pin doc warns regresses silently. |
| `__complete boguscmd ""` | Exit `0`, `ShellCompDirectiveDefault` with a cobra debug line on stderr — correct protocol behavior (shells fall back to filename completion). |
| `__complete restart ""`, `deploys list ""`, `services update --plan ""` | Empty candidate lists with `NoFileComp`: upstream registers no dynamic resource/plan completion, so completion never calls the API. Same against Render by construction. |
| `completion tcsh` | Prints the parent help, exit `0` — the upstream cobra fall-through for an unknown subcommand of a help-only parent (same class sweep 490 attributed to upstream, not bex). |

## Safety and unrun checks

- Generated scripts were written only under this run's private temp dir and never installed into a real shell profile.
- Unrun: end-to-end completion inside a live interactive shell (the protocol probes above are what the shells consume; a PTY Tab-press adds no server surface), PowerShell script parse (no `pwsh` here).
- Deploy lag means deployed revisions can precede local `main`; this sweep's surface is the local binary, so lag does not apply.

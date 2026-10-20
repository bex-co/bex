# CLI sweep 695 — completion and unknown-command handling (live, 2026-10-19)

Local CLI behavior only; the one list call was read-only.

- `bex completion bash` emits the bash completion script; `completion bogus` prints the subcommand help.
- `bex frobnicate` → `Error: unknown command "frobnicate" for "bex"` with a help hint.
- `bex services frobnicate` lists services, ignoring the positional token — the same pinned-cobra positional-arg posture classified in sweep 598, not a new finding.

No bug filed.

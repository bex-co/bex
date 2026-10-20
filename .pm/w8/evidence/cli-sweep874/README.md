# CLI sweep 874 — shell completion generation (live, 2026-10-20)

Local CLI probes (no server mutation). No new bug filed.

- `bex completion {bash,zsh,fish,powershell}` each emit their real script headers for the `bex` binary name (the launcher's rebrand reaches generated completions, unlike the upstream `render workspace set` help text recorded in sweep 825).
- An unknown shell name prints the completion subcommand help instead of generating garbage.

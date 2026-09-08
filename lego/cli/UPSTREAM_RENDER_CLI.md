# Upstream Render CLI pin

`bex` deliberately imports the upstream command package rather than copying or forking it.

- **Render release:** `v2.26.0`
- **Upstream commit:** `6c0f561f8af9d4a6cfb88f4d1845ffd18cee181a`
- **Go module version:** `v1.1.3-0.20260901190744-6c0f561f8af9`

The upstream repository's v2 tags are not valid Go-module major-version tags because its module path has no `/v2` suffix. Go therefore records the exact release commit as a pseudo-version in `go.mod`.

**Go toolchain:** v2.26.0 upgraded upstream to Go 1.27 (`85c8c2c`), so `lego/cli/go.mod` and the shared `lego/go.work` both moved to `go 1.27.0`; the three platform modules (`types`/`operator`/`backend`) and the shipped image stay on the 1.26 line (the Docker build has no workspace file). The CI jobs that build the workspace derive their Go version from `lego/cli/go.mod`, so they follow this bump automatically; `GOWORK=off` jobs (operator/backend/govulncheck) are unaffected. See [lego/CLAUDE.md](../CLAUDE.md).

## Updating the pin

1. Choose an upstream release and pin its exact commit with `go get github.com/render-oss/cli@<commit>` from this directory.
2. Update all three values above and review the upstream command/API changes. Confirm the upstream `cmd` package still exports `RootCmd` (Bex-native commands attach to it in `main.go`) and that no new upstream command collides with a Bex-native command name (`code`, `glm`, `muse`, `kimi`, `deepseek`, `upgrade`).
3. Check the update-check seams. `bex` intercepts the root `--version`/`-v` path in `main.go` (mirroring upstream's unexported `isRootVersionRequest` — re-diff that function on every pin bump) because upstream's handler compares against render-oss/cli releases behind the const `cfg.RepoURL`. A **second call site** in `pkg/tui/views/login.go` cannot be intercepted: once upstream releases something newer than this pin, `bex login`'s TUI shows a Render upgrade banner. Mitigation is bumping this pin promptly. (If upstream ever makes `RepoURL`/`InstallationInstructionsURL` vars, add the two `-X` repoints to `scripts/bex-cli-build.sh`; a PR proposing that was opened and then withdrawn by user decision 2026-08-15 — do not reopen without an explicit user decision.)
4. Re-diff the branding overlay (`lego/cli/internal/branding`). Confirm `cmd.CustomHelpTemplate` still contains `(eq .CommandPath "render")` (or update the string replace), `cliVersion` is still a cobra template func we can override, `docs` is still a root command we can retarget, and `SetupCommands` still late-adds commands so the HelpFunc re-walk remains necessary. Spot-check `bex --help`, `bex postgres --help`, and `bex blueprints --help` (`render.yaml` must stay intact).
5. Run `bash scripts/bex-cli-validate.sh`, `cd lego/cli && go test ./...`, and the live device-flow check in `scripts/bex-cli-auth-e2e.sh` when Bex auth infrastructure is available.
6. Update `docs/cli-compatibility-checklist.md` with supported-version and compatibility evidence or an explicit Bex server-side limitation.

The CI validation compares this record to `go.mod`; an unnoticed dependency bump cannot pass the launcher test workflow.

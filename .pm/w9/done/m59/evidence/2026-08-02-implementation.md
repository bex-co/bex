# m59 implementation evidence — 2026-08-02

## Completed implementation evidence

- `cli/main.go` imports `github.com/render-oss/cli/cmd` and delegates directly
  to `cmd.Execute()`; Bex-specific code is confined to
  `cli/internal/bridge`.
- `cli/UPSTREAM_RENDER_CLI.md` records Render `v2.22.0`, commit
  `d8fd7c2bb09d56beaca5df15ac2aefcb5ae5f427`, and Go pseudo-version
  `v1.1.3-0.20260721145337-d8fd7c2bb09d`. `scripts/bex-cli-validate.sh`
  compares all three against the module graph and rejects a copied `cmd/`
  tree.
- The process-level lifecycle test starts the imported executable against a
  controlled HTTP device-flow endpoint. It proves grant/token persistence to
  `~/.bex/cli.yaml` at `0600`, a forced refresh, authenticated workspace
  request, revoke/logout, and complete isolation of a pre-existing
  `~/.render/cli.yaml`. It also asserts token strings never appear in command
  output.
- CI builds release candidates for Linux/macOS amd64/arm64 and runs the
  launcher suite plus Bex's `internal/cliauth` and `internal/api` adapter
  suites. The tag workflow `bex-cli/v*` archives those binaries with SHA-256
  checksums and creates the GitHub release.

## Commands run locally

```text
bash scripts/bex-cli-validate.sh
cd cli && go test ./... -count=1
cd lego/backend && go test ./internal/cliauth ./internal/api
GOOS/GOARCH: linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 go build
```

All commands passed. Both release-workflow YAML files also parsed with Ruby's
YAML parser, and `git diff --check` passed.

## Live Bex read-only launcher evidence

An already-working local Render CLI config was copied only into a newly
created, temporary `HOME/.bex/cli.yaml` file at mode `0600`. With every
`RENDER_*` variable explicitly removed and `BEX_HOST=https://api.bex.co/v1/`,
the locally built `bex` binary successfully ran `workspaces -o json` (a
non-empty `tea-…` workspace list) and `services -o json` for that workspace.
The source Render config's SHA-256 was identical before and after, no
`HOME/.render/cli.yaml` existed in the temporary home, and every temporary
credential-bearing file was removed individually after the check.

This proves the native launcher's authenticated Bex REST path and Bex-config
isolation without printing, persisting, or revoking a real human credential.
It deliberately does **not** substitute for the disposable-identity browser
login, forced refresh, and revoke evidence below.

## Live device-auth evidence

On the restored isolated `dev-9` Bex/Kratos/Hydra stack, the disposable local
identity path passed:

```text
CREATE_TEST_IDENTITY=1 \\
  BEX_API_URL=http://localhost:54090 \\
  HYDRA_ADMIN_URL=http://localhost:52090 \\
  KRATOS_ADMIN_URL=http://localhost:57090 \\
  bash scripts/bex-cli-auth-e2e.sh
```

The runner drove the real dashboard device-verification flow, then proved
`~/.bex/cli.yaml` is mode `0600`, forced an automatic refresh and authenticated
`services -o json` request, logged out, and verified both the former bearer
and refresh token were rejected. Its isolated temporary `~/.render/cli.yaml`
remained unchanged throughout. The runner contains no credential-printing path.

## Release-gate and cleanup evidence

- `scripts/bex-cli-validate.sh` locks the Render release, commit, module
  pseudo-version, import boundary, and release-build version stamp. Both CI
  workflows invoke it before building artifacts.
- The release workflow now runs the same `internal/cliauth` and `internal/api`
  adapter tests as pull-request CI. `scripts/bex-cli-build.sh` supplies the
  one Linux/macOS amd64/arm64 build matrix to both workflows.
- The launcher bridge treats blank `RENDER_*` inputs as unset, so inherited
  empty variables cannot send Bex back to Render defaults. Its process tests
  cover stored Bex credentials, temporary Bex bearer credentials, a missing
  Bex credential with a protected Render config, device login, refresh, and
  logout.
- Browser test credentials travel on standard input, never process arguments;
  failure diagnostics redact URL query strings and page contents.
- The release workflow's archive layout was also exercised locally for all
  four targets; every generated `tar.gz` matched its SHA-256 checksum.

## Deliberately incomplete release gate

- No `bex-cli/v*` GitHub tag/release has been created. Creating it would be an
  external publication action, so m59 remains open until a release owner
  authorizes and performs (or directs) that step.

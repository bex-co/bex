# Upstream Render CLI pin

`bex` deliberately imports the upstream command package rather than copying or forking it.

- **Render release:** `v2.22.0`
- **Upstream commit:** `d8fd7c2bb09d56beaca5df15ac2aefcb5ae5f427`
- **Go module version:** `v1.1.3-0.20260721145337-d8fd7c2bb09d`

The upstream repository's v2 tags are not valid Go-module major-version tags because its module path has no `/v2` suffix. Go therefore records the exact release commit as a pseudo-version in `go.mod`.

## Updating the pin

1. Choose an upstream release and pin its exact commit with `go get github.com/render-oss/cli@<commit>` from this directory.
2. Update all three values above and review the upstream command/API changes.
3. Run `bash scripts/bex-cli-validate.sh`, `cd cli && go test ./...`, and the live device-flow check in `scripts/bex-cli-auth-e2e.sh` when Bex auth infrastructure is available.
4. Update `docs/cli-compatibility-checklist.md` with supported-version and compatibility evidence or an explicit Bex server-side limitation.

The CI validation compares this record to `go.mod`; an unnoticed dependency bump cannot pass the launcher test workflow.

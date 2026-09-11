# w5 · m94 — CLI telemetry sender completeness: bex release version, launcher-native commands, exec-based provider launches

**Worker:** worker5 **Goal:** every CLI telemetry row bex-api stores carries the bex launcher release next to the upstream pin, and the launcher-owned commands the upstream sender never sees — root `--version`, `bex upgrade`, and the `bex code` / `bex glm` provider launches that `exec` away — emit one Render-shaped event each under the same consent rules, so the CLI usage board can measure release adoption and agent-launch usage instead of labeling them as gaps. **Status:** todo (t001-t008 done; t009 closeout pending live board verification)

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------- | --- | ---------- |
| [t001](done/t001.md) — **DONE** | Launcher: `X-Bex-CLI-Version` header on every bex-api request via a default-transport wrapper               | 30m | —          |
| [t002](done/t002.md) — **DONE** | bex-api: persist a bounded `bex_version` from the header; expose it in `cli_analytics.events`               | 45m | t001       |
| [t003](done/t003.md) — **DONE** | Launcher-native events: root version, upgrade, and provider launches before `exec`, consent-gated           | 60m | t002       |
| [t004](done/t004.md) — **DONE** | CLI usage board: bex-release filter + installations-by-release panel; label provider launches; retire caveat | 30m | t003       |
| [t005](done/t005.md) — **DONE** | End-to-end on dev-5 with a tree-built launcher at the pinned upstream version                               | 30m | t004       |
| [t006](done/t006.md) — **DONE** | Render parity                                                                                               | 30m | t005       |
| [t007](done/t007.md) — **DONE** | Simplify                                                                                                    | 30m | t006       |
| [t008](done/t008.md) — **DONE** | Test coverage                                                                                               | 45m | t006       |
| t009 | Closeout                                                                                                    | 15m | t008       |

## Definition of done

- A telemetry row produced by a launcher built with `scripts/bex-cli-build.sh` carries `cli_version` = the pinned upstream version **and** `bex_version` = the bex tag; a plain `go build` stores `dev`. An unmodified upstream `render` binary pointed at bex still ingests with `bex_version = ''` (never rejected).
- `bex --version`, `bex upgrade`, and one `bex glm` / `bex code` launch each produce exactly one event; no upstream command produces a second event.
- `DO_NOT_TRACK`, `BEX_CLI_DISABLE_ANALYTICS`, and an explicit `RENDER_CLI_DISABLE_ANALYTICS` suppress the new emitters exactly as they suppress the upstream sender; no event body ever carries command arguments, prompts, or paths.
- The live CLI usage board at obs.bex.co filters by bex release and shows installations per release; provider launches appear under their own command labels; the m93 "coverage boundary" wording is removed wherever the measurement now exists.
- `docs/bex-cli.md` § CLI usage telemetry and the `docs/cli-compatibility-checklist.md` telemetry row describe the header, the stored field, and the launcher-native events.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` 2026-09-10 proposal 2, from the `w5/m93` closeout's recorded coverage boundary: "Bex release-version instrumentation and successful exec-based provider-launch telemetry require separate sender work" and "root version requests, logged-out/early setup failures and successful exec-based provider launchers have collection gaps" ([done/m93/README.md](../done/m93/README.md)).
- **Facts verified 2026-09-10:** `cli_telemetry_events.cli_version` stores only the upstream pin (`store/migrations/0113_cli_telemetry_events.up.sql`); `main.bexVersion` is injected by `scripts/bex-cli-build.sh` (`-X main.bexVersion=`) but never reaches bex-api. The upstream generated client is built as `&http.Client{}` with a nil Transport (`render-oss/cli` `pkg/client/client.go:41`), so it rides `http.DefaultTransport`; the analytics sender re-execs the same binary through `os.Executable()` (`pkg/analytics/subprocess.go:86`), so a transport wrapper installed in the launcher's `main()` applies to the detached send as well — no fork. `bex code` / `bex glm` replace the process with `syscall.Exec` (`lego/cli/internal/code/launch.go:116`), so upstream's post-run analytics hook never fires for them; root `--version` exits before `cmd.Execute()` (`lego/cli/main.go`).
- **Goal linkage:** ADR008's AI-native thesis — the coding commands are the agent-launch path of the CLI and are invisible today; ADR088 CLI usage board (product analytics); ADR058 release train (release adoption of `bex-cli/v*` is unmeasurable without the field).
- **Expected outcome:** operators can answer "which bex releases are in use" and "how often do people launch an agent through bex" from the existing board, and the self-update channel (`w4/m37`) has a measured adoption curve.
- **Why now:** m93 shipped 2026-09-10 with this gap labeled on the board itself; the next `bex-cli/v*` train can carry the sender, and every release until then adds unattributable rows to a 90-day retention window.
- **Render parity included:** the milestone extends the REST telemetry endpoint with a header-derived field. Upstream exposes telemetry over REST only, so — as `w5/m92` decided — GraphQL/MCP twins are deliberately out of scope; the parity task confirms the exact upstream body is still accepted and records the header as a bex extension in the checklist.
- **Anti-goal check:** `.pm/DO_NOT_DO.md` forbids a first-party/forked CLI. This milestone touches only the launcher's process configuration (a transport wrapper) and its own added cobra commands; `github.com/render-oss/cli` stays an unmodified dependency.

## Evidence

**Local stub-server verification (2026-09-10, tree-built launcher).** A launcher built the way releases are built (`-X …cfg.Version=2.27.0 -X main.bexVersion=…`) was pointed at a local stub that logs headers and event bodies.

- **t001 — header reaches both request classes.** `bex workspaces -o json` produced `GET /v1/owners?limit=100` **and** the detached `POST /v1/cli-telemetry-events`, each carrying `X-Bex-CLI-Version`. This is the load-bearing proof that wrapping `http.DefaultTransport` covers the analytics subprocess, which re-executes the binary. `User-Agent` stayed `render-cli/2.27.0 (macOS - 26.5.1)`, byte-identical to upstream's.
- **t003 — which paths were actually silent.** Measured rather than assumed: `bex upgrade` **already** emits through upstream's post-run hook (it is an ordinary cobra command on the imported root), so it needed no work. Only root `--version` and the exec-based provider launches were silent. One event each after the fix, with no path emitting twice:

  | invocation | `command` | `completion_kind` | emitter |
  | --- | --- | --- | --- |
  | `bex workspaces -o json` | `bex workspaces` | `success` | upstream hook |
  | `bex --version` | `bex` | `version` | launcher-native (new) |
  | `bex upgrade` | `bex upgrade` | `execution_error` | upstream hook |
  | `bex glm` (stub `claude`) | `bex glm` | `success` | launcher-native (new) |
  | `bex code` | `bex code` | `success` | upstream hook |

- **No double-count, no gap.** `bex glm` with no `claude` on `PATH` emits exactly one event (`execution_error`, via the upstream hook) because the process is never replaced.
- **Opt-outs.** `DO_NOT_TRACK=1`, `BEX_CLI_DISABLE_ANALYTICS=1`, and `RENDER_CLI_DISABLE_ANALYTICS=1` each added **zero** events across a provider launch and a `--version` run. The new emitters delegate to upstream's `analytics.New(...).Send`, so consent, backoff, and installation identity are upstream's, not reimplemented.
- **Test-harness hazard worth recording.** An early run showed `bex upgrade` "not emitting". Root cause was the harness, not the code: `bex upgrade` really self-updated the test binary to the released `v0.2.1` (which predates this work), so later invocations were a different program. Re-verified with the build version set above the latest release so `upgrade` no-ops instead of replacing itself.

**Mutation checks (tests fail when the behavior is removed).** Dropping the host allowlist in the transport makes the third-party-host test fail; mutating the caller's request instead of a clone fails the RoundTripper-contract test; deleting the pre-exec hook call fails the ordering test.

**Suites.** `cd lego/cli && go test ./...` green (7 packages). `cd lego/backend && go test ./internal/clitelemetry/` green; `./internal/store/` telemetry + migration-0116 rollback tests green against a disposable PostgreSQL 17 container. `python3 -m unittest scripts.test_cli_analytics` green (11 tests).

## Simplify pass (t007)

Three parallel reviews (reuse, quality, efficiency) ran over the whole change set. They found one defect that would have broken the production rollout, plus one real hot-path regression. Both are fixed and guarded by tests.

**Deployment blocker — the analytics view could not be re-applied to production.** The new `bex_version` projection was inserted mid-list in `scripts/cli-analytics.sql`. `CREATE OR REPLACE VIEW` can only **append** columns, so on any cluster that already holds the m92 view — production — the bootstrap script would abort its whole transaction with `cannot change name of view column "os" to "bex_version"`. Reproduced against PostgreSQL 17 by applying the previous generation and then this one. The existing test could not catch it because it applied the *new* script twice, so both runs agreed on column order. Fixed by appending the column, and guarded by `test_view_columns_are_append_only`, which pins the released column list as a prefix; mutation-checked by moving the column back mid-list and watching the test fail. (A first attempt at the guard derived the "previous" generation from the current script, which was self-defeating — it inherited the same mid-list insertion and passed. Replaced with the pinned prefix.)

**Hot-path regression — opted-out users paid for telemetry they had declined.** `send` built the API client before resolving consent, and upstream's constructor reads the CLI config and can spend up to five seconds on a synchronous OAuth refresh. Both callers run where that is unaffordable: the last statement before the process is replaced by the launched agent, and `--version`. Consent is now resolved first (environment-only, free), so an opted-out user pays nothing; `TestSendChecksConsentBeforeBuildingAClient` pins the ordering. The `EmitLaunch` doc no longer claims the path is free for everyone.

**Two comments asserted guarantees the code did not provide.** The duplicated header constant claimed "the rename guard is the parity test" — no such test existed, so renaming either side would have silently stopped attributing releases. Both modules now pin the literal (`TestBexVersionHeaderNameIsPinned` and its launcher twin). The telemetry package doc claimed full parity with upstream's gating; in fact upstream's one-time-disclosure gate lives in Cobra's post-run path and is *not* applied to these two paths. The doc now says so and points at the runbook where the divergence was already recorded.

**Applied cleanups.** `sync.OnceValue` for host resolution (the repo's existing idiom) and structural idempotence for the transport install, removing two package-level mutable vars and the test-only reset they forced; dead `nil` branches dropped from `RoundTrip` and `execClaude`; the two pre-existing launcher telemetry tests folded onto the new `telemetryEnv`/`awaitEvents` helpers and the superseded `first` deleted; a `read_as_reader` helper in the Python suite; a weak `assertGreaterEqual` replaced with the exact expected count its own comment promised; a positive-half guard so a future panel cannot silently ignore the release filter; comments moved out of two struct literals that were re-aligning 22 untouched fields; and five unrelated em dashes restored after a JSON round-trip had escaped them.

**Considered and rejected.** A reviewer proposed also sanitizing the upstream `cli_version`, which shares the same untrusted body and dashboard axis. Doing so broke two m92 tests that deliberately encode "ingest stays liberal — truncate, never reject" for every body field. Tightening a shipped field would silently drop data stored today, which is a change to m92's rule and needs its own decision, not a drive-by in m94. The asymmetry is instead documented and pinned by `TestVersionAxesAreDeliberatelyAsymmetric` so it is not mistaken for an oversight.

## Gates (t008)

- `make lint` — 0 issues across all four modules (it caught three unchecked `Close` calls in a new test, since fixed).
- `cd lego/cli && go test ./...` — 7 packages green.
- `cd lego/backend && go test ./...` — 62 packages green against a disposable PostgreSQL 17. One local-only failure, `TestGatewayScopedRoleAllowsOwnSurfaceDeniesTheRest`, is a harness artifact and not a regression: PostgreSQL roles are cluster-wide, so grants left in the extra databases this session created block the test's own role cleanup. It passes once those are dropped, and CI gives each run a dedicated server. Verified, not assumed.
- `python3 -m unittest scripts.test_cli_analytics` — 16 tests green, executing every committed panel query.
- `bash scripts/gitops-validate.sh` — PASS.

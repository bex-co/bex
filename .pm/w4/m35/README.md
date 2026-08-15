# w4 · m35 — bex CLI distribution: first release, install script, signing, Homebrew tap

**Worker:** worker4 **Goal:** installing the bex CLI becomes one command (`curl | sh` or `brew install bex-co/tap/bex`), backed by a real published `bex-cli/v0.1.0` release with signed checksums — activating the update-notice channel m34 built. **Status:** in progress (t001–t003 done; t004 blocked on `/ship` + `BEX_TAP_PUSH_TOKEN`)

## Tasks (in order)

| id   | title                                                                        | est | depends_on |
| ---- | ---------------------------------------------------------------------------- | --- | ---------- |
| t001 | `scripts/install-bex.sh`: one-line installer with checksum verification — **DONE**      | 45m | —          |
| t002 | cosign keyless signing of `checksums.txt` in `cli-release.yml` — **DONE**               | 30m | t001       |
| t003 | Homebrew tap: `bex-co/homebrew-tap` + formula auto-push on release — **DONE**           | 60m | t002       |
| t004 | First release `bex-cli/v0.1.0`: tag, watch, verify every channel end to end  | 45m | t003       |
| t005 | Simplify — `/simplify` over the code this milestone changed                  | 30m | t004       |
| t006 | Test coverage — installer/formula-render logic that can be tested offline    | 30m | t005       |
| t007 | Closeout                                                                     | 15m | t006       |

## Definition of done

A machine with none of our tooling can run the documented `curl -fsSL … | sh` and get a working `bex` on PATH whose `bex -v` reports `v0.1.0`; `brew install bex-co/tap/bex` installs the same binary; the `bex-cli/v0.1.0` GitHub release carries four platform archives, `checksums.txt`, and a cosign `.sigstore.json` bundle verifiable keylessly; the release workflow pushes the rendered formula to `bex-co/homebrew-tap` using `BEX_TAP_PUSH_TOKEN` (step skips gracefully when the secret is absent); docs cover install + verify; `.env.example` mirrors the new secret name.

## Source + Goal linkage

- **Source:** 2026-08-15 distribution research (Render CLI's install surface as the reference; GoReleaser evaluated and deliberately not adopted — its monorepo tag-prefix support is Pro-only and our hand-rolled release chain already covers its core outputs). User decisions: first version `0.1.0`, tap repo creation authorized, `BEX_TAP_PUSH_TOKEN` to be provided by the user.
- **Goal linkage:** the CLI is a first-class product surface (docs/bex-cli.md); m34 shipped the update-notice channel but zero releases exist, so there is nothing to notice, no way to install without a Go toolchain, and `w4/030`'s `bex upgrade` has no artifacts to consume.
- **Expected outcome:** `bex` installable by anyone in one command; every future `bex-cli/v*` tag automatically publishes archives + signature + formula.
- **Why now:** m34's channel is dark until a release exists; the first tag also end-to-end validates m34's version injection. Render parity closing task omitted: distribution infra only — no REST/GraphQL/MCP/dashboard surface change.

---
name: release
description: Cut a versioned release of a bex component (today: cli) — compute/confirm the version, push the tag, watch the release workflow to green, and verify every distribution channel. Use when the user asks to release, publish, or bump the CLI (or a future component). /ship lands code; /release mints versions.


allowed-tools: Bash(git:*), Bash(gh:*), Bash(cosign:*), Bash(brew:*), Bash(scripts/install-bex.sh:*), Bash(curl:*), AskUserQuestion
---

# Task: Release a bex component

`/release <component> [patch|minor|major|X.Y.Z]` — component today: `cli` (tag prefix `bex-cli/v`). `platform` is reserved for the ADR058 `bex/v*` train and must be rejected until that exists.

**Invariant (ADR058):** code flows into `main` only through `/ship`; versions exist only as tags minted here. This skill never commits, never touches the working tree, and never re-uses or moves an existing tag. A pushed tag is a public, immutable release.

## Step 1 — Preconditions (all must hold; stop with a clear message otherwise)

1. Working tree clean (`git status --porcelain` empty) and current branch `main`.
2. `HEAD == origin/main` after `git fetch origin main` — releases are cut only from shipped code.
3. CI for HEAD is green: `gh run list --commit $(git rev-parse HEAD)` shows no failed/in-progress required runs. If runs are still in progress, wait for them.

## Step 2 — Determine the version

1. Last tag: highest existing `bex-cli/v*` by semver (`git tag -l 'bex-cli/v*'`; also `git fetch --tags` first). No tags yet ⇒ this is the first release; default `0.1.0` unless an explicit version was given.
2. List commits since that tag touching the component's paths — for `cli`: `lego/cli/**`, `scripts/bex-cli-*`, `scripts/install-bex*`, `.github/workflows/cli-release.yml`. If there are none, say so and stop (nothing to release) unless the user explicitly insists.
3. Suggested bump from Conventional Commits over those commits: any `!`/`BREAKING CHANGE` → major (while on `0.x`, propose minor instead and say why), else any `feat` → minor, else patch.
4. If the user passed a level or exact version, that wins. Otherwise present the suggestion plus the commit list and **ask for confirmation** (`AskUserQuestion`) — the confirmed version is the authorization to publish.

## Step 3 — Tag and push

```bash
git tag -a "bex-cli/vX.Y.Z" -m "bex CLI vX.Y.Z"
git push origin "bex-cli/vX.Y.Z"
```

## Step 4 — Watch the release workflow to green

The tag triggers `release (bex CLI)` (`cli-release.yml`). Watch it like `/ship` watches deploys (`gh run watch <id> --exit-status`); rerun genuine infra flakes (max 2). If it fails on a real defect: **do not delete or move the tag**. Diagnose, report, get the fix landed via `/ship`, then cut the next patch version here. If the failure happened before the GitHub release was created, deleting the never-published tag is permitted with user confirmation.

## Step 5 — Verify every channel

1. **Release assets:** `gh release view bex-cli/vX.Y.Z` lists 4 platform archives + `checksums.txt` + `checksums.txt.sigstore.json`.
2. **Signature:** download `checksums.txt` + bundle; `cosign verify-blob checksums.txt --bundle checksums.txt.sigstore.json --certificate-identity-regexp 'github.com/bex-co/bex/' --certificate-oidc-issuer https://token.actions.githubusercontent.com` (skip with a note if `cosign` isn't installed locally).
3. **Installer:** run `scripts/install-bex.sh` with `BEX_INSTALL_DIR` pointing at a temp dir; assert the installed `bex -v` reports the new version.
4. **Homebrew:** confirm `Formula/bex.rb` in `bex-co/homebrew-tap` now carries the version (the workflow step is gated on `BEX_TAP_PUSH_KEY`; if it was skipped, flag it). Optionally `brew install bex-co/tap/bex && bex -v && brew uninstall bex` — uninstall to avoid shadowing a dev binary earlier in PATH.
5. **Update notice:** the temp-installed binary's `bex -v` against the live API should print "You are using the latest version" (or the upgrade hint if a newer tag already exists).

## Step 6 — Report

One line per outcome: tag, workflow run URL/verdict, and each channel's verification result. Then update any tracking the user's flow expects (e.g. an open `.pm` task for the release).

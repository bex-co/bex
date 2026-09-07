---
name: release
description: Autonomously release a bex component (today cli) — sync shipped main, choose the version, publish the tag, recover routine failures, and verify every distribution channel. Use when the user asks to release, publish, or bump the CLI. /ship lands code; /release mints versions.
allowed-tools: Bash, Read, Glob, Grep
---

# Task: Release a bex component

`/release <component> [patch|minor|major|X.Y.Z]` (Codex: `$release`) — today `cli`, tag prefix `bex-cli/v`. Reject `platform` until the ADR058 `bex/v*` train exists.

**Authorization:** a release request authorizes syncing shipped code, choosing the version by the rules below, pushing a new tag, running the release workflow, and verifying channels. Announce the version and source SHA, then proceed without confirmation. Honor explicit preview-only, no-publish, or exact-version constraints. A request only to edit this skill is not an instruction to publish.

**Invariants (ADR058):** release only code already on `origin/main`. Code changes land through `/ship`; this skill does not commit or push branches in the bex repository. Publishing the Homebrew formula remains part of the authorized release. Preserve local work. Never delete, move, or reuse an existing remote tag, even if it has no GitHub Release. Recover with a fresh version instead.

## 1 — Prepare the source automatically

1. Fetch `origin main` and tags. On clean `main` that is behind, fast-forward with `git merge --ff-only origin/main`; do not stop to ask. If the checkout is dirty, on another branch, ahead, or diverged, use a temporary detached worktree at `origin/main`. Do not stash, reset, rebase, include local commits, or disturb another session's work. Keep temporary verification artifacts outside the user's checkout.
2. Record the fetched main SHA as the release candidate. Inspect its full commit message **before tagging** for GitHub skip instructions: `[skip ci]`, `[ci skip]`, `[no ci]`, `[skip actions]`, `[actions skip]`, or `skip-checks: true`. These also suppress tag-push workflows; a successful CodeQL run alone does not establish release readiness.
3. If the tip has a skip instruction, inspect the intervening commits and diff to the nearest first-parent ancestor without one. Automatically select that ancestor only if every intervening change is a generated deployment image-reference pin, with no source, dependency, build, installer, or workflow changes. Verify the diff itself, not just commit subjects. Record this source choice and keep the checkout at main (or use a detached worktree); tag the selected SHA explicitly. For any broader difference, prepare the necessary fix and stop at the `/ship` boundary rather than silently omitting shipped code.
4. Determine required checks from repository rules and the component's workflows. Inspect individual jobs on the selected SHA, including reusable test jobs inside `deploy.yml`. All required checks and relevant tests must pass; wait for pending checks and retry genuine infrastructure flakes at most twice. For CLI, cover launcher/pin validation, installer checks, and backend device-flow/API tests as specified by the current workflows. Run missing applicable checks from the selected source locally where supported. No runs, skipped checks, or a docs-only green run are not proof that tests passed.
5. Platform image builds, Helm mirroring, and cluster rollouts are not CLI release gates unless repository rules explicitly require them. Once the required checks and relevant test jobs pass, proceed without waiting for unrelated deployment steps. Do not ignore a required failure. For real code defects, diagnose and prepare a fix; land it through `/ship` only when separately authorized.
6. Re-fetch main before publication. If it advanced with release-relevant changes, update the candidate and repeat the applicable checks. If only unrelated changes or generated pins advanced, keep the validated candidate after verifying it is still an ancestor of `origin/main` and release inputs are unchanged. Do not chase deployment write-back commits indefinitely.

## 2 — Select the version without asking

1. List remote `bex-cli/v*` tags and GitHub Releases. Use the highest stable **published release** as the change baseline; tags without published releases still reserve their version but do not mean changes have shipped. Inspect incomplete releases and active release runs before starting duplicate work. Distinguish an actual missing release from an API/authentication error.
2. List commits since the baseline touching `lego/cli/**`, `scripts/bex-cli-*`, `scripts/install-bex*`, and `.github/workflows/cli-release.yml`. Also inspect shared build/dependency inputs used by these paths. If there are no unreleased changes or incomplete channels to finish, report nothing to release and stop unless the user explicitly requested an empty release.
3. First release defaults to `0.1.0`. Otherwise use Conventional Commits: `!` or `BREAKING CHANGE` means major (minor while on `0.x`); any `feat` means minor; otherwise patch. A user-supplied bump level or exact version wins. Report the selected version with a concise summary of included changes; **do not ask for release confirmation**.
4. If the computed version is already reserved by an abandoned tag, select the next patch above both the computed version and the highest existing version. Example: published `0.1.0`, feature changes suggest `0.2.0`, but an orphan `0.2.0` tag exists → publish `0.2.1`. Do not advance again if the matching release is actively running; resume monitoring it. For an existing completed release, verify it instead when that satisfies the user's exact request.
5. An explicit exact version is a constraint: do not silently replace it with another version or point its tag elsewhere. If that version is occupied by a conflicting or unrecoverable attempt, explain the conflict and ask only for the necessary version decision.

## 3 — Publish and recover

1. Recheck remote tag availability and the selected source's ancestry, skip instructions, and required checks. Use an annotated tag targeting the recorded SHA explicitly:

   ```bash
   git tag -a "bex-cli/v${release_version}" "$release_sha" -m "bex CLI v${release_version}"
   git push origin "refs/tags/bex-cli/v${release_version}"
   ```

2. If the push result is uncertain or another publisher raced, inspect the remote tag before retrying. Continue only if it resolves to the intended source, or apply the version rules above. Never force-push a tag or treat a network error as proof publication did not happen.
3. Find `release (bex CLI)` (`cli-release.yml`) by the exact tag and SHA. Poll for up to two minutes for workflow creation, then diagnose a missing run rather than waiting indefinitely. Use `workflow_dispatch` only if the workflow already supports releasing that exact tag. If an orphan tag cannot start a run, preserve it and use a fresh patch with a valid source, subject to any exact-version constraint. Limit automatic orphan recovery to one replacement version per invocation; repeated non-starts require resolving the trigger problem before minting more tags.
4. Watch the release workflow to completion (`gh run watch <id> --exit-status`). Keep tool waits short enough to accept user input and provide progress. Retry proven infrastructure flakes at most twice; do not retry deterministic failures unchanged. Missing credentials or environment approvals require the authorized owner; do not weaken protection rules to proceed.
5. Once assets are published, do not rerun a job that rebuilds and overwrites them. Inspect failed steps: recover an incomplete downstream channel using the existing signed assets through a channel-only job or the existing channel tooling when authorized. If recovery requires new source code, prepare the fix and use `/ship` when authorized, then release a new patch. Never call a partial release complete.

## 4 — Verify every channel

1. **Assets:** `gh release view bex-cli/vX.Y.Z` must list four archives (Linux/macOS × amd64/arm64), `checksums.txt`, and `checksums.txt.sigstore.json`. Verify these belong to the intended tag/source.
2. **Signature:** download checksums and the bundle to a temporary directory. If cosign is missing, install it using the available trusted package manager when permitted. Verify with the CLI release workflow identity and GitHub OIDC issuer; use the policy in `scripts/install-bex.sh`. Do not bypass signature verification to make the installer pass.
3. **Installer:** run `scripts/install-bex.sh` from the selected source with `BEX_VERSION=X.Y.Z` and `BEX_INSTALL_DIR` set to a temporary directory. Assert the resulting `bex -v` reports the new version. Also verify default latest-release discovery selects this release (or a newer concurrently published version).
4. **Homebrew:** check `Formula/bex.rb` in `bex-co/homebrew-tap` for the intended version, asset URLs, and matching checksums. Check whether the publish step was skipped because `BEX_TAP_PUSH_KEY` was missing; report an incomplete channel instead of success. An optional local install must not replace or uninstall a preexisting user binary.
5. **Update notice:** run the temporary binary's `bex -v` with update checks enabled against the live API and a fresh temporary cache, avoiding stale local results. Expect “You are using the latest version” or an accurate upgrade hint if another release overtook this one.

## 5 — Finish or stop only at a real boundary

Report tag and source, workflow URL/verdict, and each channel's verification result. Update release tracking within the user's authorized scope only after verification; do not mark an unfinished release done.

Routine sync, default version selection, normal CI waits, isolated worktrees, and bounded transient retries do not need confirmation. Stop only when there is nothing to release, the component is unsupported, or further progress needs unavailable credentials/approval, a source fix landed through `/ship`, an exact-version decision, or an action that would violate the invariants. Before a blocked stop, complete available diagnosis and reversible preparation, state the concrete remaining action, and cite the applicable boundary. Keep prior authorization; do not ask again for a version already authorized.

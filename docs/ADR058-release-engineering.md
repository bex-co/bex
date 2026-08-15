# ADR058 — One shared platform version (release engineering)

**Status:** Accepted (user decision 2026-08-15). CLI mechanics instantiated by `w4/m34`–`m35`.

## Context

The monorepo ships things with different consumers: platform services we run ourselves (continuously deployed via `deploy.yml` → GitOps digest pin → Argo), and user-installed artifacts (the `bex` CLI today; mobile later). Building the CLI's release channel forced the question of how the repo's components are versioned — per component, or together.

## Decision

**bex converges on one shared version — Kubernetes-style lockstep, including the CLI.** Like kubectl and the control plane, the version number itself is the compatibility statement. The gh model (client and server versioned independently, forever) is rejected.

- **The train:** `bex/vX.Y.Z` covers operator + bex-api + ssh-gateway + dashboard + the self-host install artifact (Helm/kustomize). These already build and test as one unit (`lego/` is one image with multiple entrypoints); the shared version recognizes that.
- **Launch = self-host delivery (≈ 1.0).** A shared version stands for upgrade obligations — cross-version upgrade paths, CRD schema and control-plane DB migrations. The train launches when we take those on, not before.
- **Until launch:** services carry no version tags — their identity is image digest + git SHA in the GitOps pin ledger. The CLI rides `bex-cli/v0.x` independently; **0.x explicitly means "pre-convergence"**. At launch it jumps to the shared version, accepts kubectl-style empty releases, and gets a documented skew policy (e.g. ±1 minor, enforceable by the m34 update notice).
- **After launch, both identities coexist:** our own cluster keeps deploying continuously from `main` by digest; each platform release casts a tested digest set into a version for external operators. The train must not slow our own cadence.
- **Exemptions:** mobile (store/OTA cadence cannot lockstep) and platform-pinned assets (e.g. the agent-sandbox image). They declare a compatibility **range** against the platform version instead of sharing the number.
- **Internal dependencies are never versioned** (`lego/types` etc.): everything in the workspace is consumed at the same commit — atomic cross-component commits are the point of the monorepo.

## Release mechanics (survive convergence unchanged)

Established by the CLI and reused by the train: the tag is the **only** version source (build-time injection, no VERSION files); releases are tag-triggered (`cli-release.yml` template: test gates → build matrix → `checksums.txt` → keyless cosign bundle → GitHub release → channels) and immutable; the shared repo's `/releases` is partitioned by tag prefix, so consumers always filter by prefix (never `/releases/latest`) and asset URLs carry the encoded `%2F`. Versions are minted only by the `/release` skill (preconditions → suggested bump → confirm → tag → watch → verify channels); `/ship` lands code and never tags. Staleness reminder + tag ruleset: `w4/032`.

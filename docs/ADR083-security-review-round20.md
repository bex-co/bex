# ADR083: Security review round 20 — self-hosted GitHub Actions disposition

- **Status**: Accepted (2026-08-23)
- **Trigger**: operator decision to migrate every workflow from GitHub-hosted (`ubuntu-latest` / `ubuntu-24.04-arm`) to self-hosted runners (`[self-hosted, Linux, ARM64]`)
- **Lineage**: twentieth pass in the ADR028 → … → ADR080 lineage; re-confirms ADR080 residuals (onbex.co PSL, secretless-build split) unchanged

## Summary

All repository workflows now target self-hosted runners. That trades GitHub-hosted runners' per-job ephemeral VMs for persistent hosts the operator owns — a deliberate custody shift, not a code defect. The residual risks below are **accepted** for bex.co production CI, with the mitigations and operator obligations listed. They do not reopen the removed static-CIDR firewall (`.pm/DO_NOT_DO.md`); a Tailscale/WireGuard second layer remains optional follow-up, and self-hosted runners are now the documented path to it ([ADR019-infra-credentials.md](ADR019-infra-credentials.md) §Decision 5).

| # | Risk | Severity | Disposition |
| --- | --- | --- | --- |
| 1 | Self-hosted runners are persistent — job isolation is weaker than GitHub-hosted ephemeral VMs | high | **Accepted residual** — operator hardens hosts; ephemeral-runner follow-up optional |
| 2 | Untrusted `pull_request` jobs can execute repository-controlled code on the runner fleet | high | **Accepted residual** — fork PRs must stay off self-hosted; same-repo PRs rely on branch protection + review |
| 3 | Production credentialed jobs (`deploy`, `infra`, `app-cluster`, `snapshot`, `openbao-restore-drill`, `cli-release`) share one runner label with lower-trust jobs unless pools are split | high | **Accepted residual** — single `[self-hosted, Linux, ARM64]` pool for now; split pools is follow-up |
| 4 | Runner host compromise equals platform takeover (HCLOUD, SSH admin.conf, OpenBao root + Shamir quorum, auth bootstrap secrets, `contents: write` git push) | critical | **Accepted residual** — bounded by ADR080 environment gates + main-ref guards; host custody is the new trust anchor |
| 5 | Docker layer / toolchain cache poisoning across sequential jobs | medium | **Accepted residual** — `docker/setup-buildx-action` + pinned downloads; no shared cache hardening in YAML |
| 6 | ARM64 runner fleet required | low | **Accepted residual** — operator provisions Linux ARM64 self-hosted runners with Docker |
| 7 | onbex.co PSL cross-tenant cookie poisoning | low | **Accepted residual** (fifteenth report) — unchanged from ADR080 #8; `.pm/DO_NOT_DO.md` `#PSL` |

## Context — what changed

Every workflow under `.github/workflows/` and `lego/operator/.github/workflows/` now uses:

- `runs-on: [self-hosted, Linux, ARM64]` for every workflow job (matches the operator's ARM64 runner fleet). Docker Buildx still emits `linux/amd64` platform images for production deploys.

Workflows that previously relied on GitHub-hosted preinstalls (`kubectl`, `helm`, `yq` on `ubuntu-latest`) now install pinned tool versions in-job (`azure/setup-kubectl`, `azure/setup-helm`, checksum-pinned `yq` curl installs in `gitops.yml` / `clusterapi-validate.yml`; `deploy.yml` / `app-cluster.yml` / `openbao-restore-drill.yml` carry `setup-kubectl` where they previously assumed the hosted image).

ADR080's production environment gates (`production-deploy`, `production-cluster`, `production-restore`, plus the pre-existing `production-infra`, `production-snapshot`, `production-release`) and main-branch ref guards remain the **workflow-side** controls. They do not substitute for runner isolation.

## Finding 1 (high) — loss of ephemeral VM isolation

GitHub-hosted runners discard the VM after each job. Self-hosted runners reuse the same kernel, Docker daemon, and filesystem unless the operator provisions ephemeral runners (ARC with `ephemeral: true`, VM-per-job, etc.).

**Accepted.** Production CI for bex.co is operator-custodied infrastructure (same class as the Hetzner app cluster itself). Host patching, runner version upgrades, and disk hygiene are operator obligations. Ephemeral self-hosted runners remain a follow-up if a stricter isolation bar is wanted without returning to GitHub-hosted.

## Finding 2 (high) — untrusted PR code on self-hosted runners

Sixteen workflows run on `pull_request` (tests, lint, credential-less `infra validate`, `gitops render`, etc.). A same-repo branch contributor can execute arbitrary code in those jobs. On self-hosted hardware that code can probe the local network, read leftover files from prior jobs, or poison caches consumed by later jobs.

**Accepted**, with mandatory operator settings:

- **Fork PRs must not use self-hosted runners.** In repository Settings → Actions → General, restrict self-hosted runners so workflows from fork PRs cannot schedule on them (GitHub's "Require approval for first-time contributors" is necessary but not sufficient — fork workflows must be blocked from the self-hosted pool entirely).
- Same-repo PRs continue to rely on branch protection, required reviews, and the existing credential-less split for `infra.yml` `validate` (no cloud token, no `terraform plan` against real state).

## Finding 3 (high) — shared runner pool across trust levels

Low-trust jobs (PR tests) and high-trust jobs (`build-and-deploy` with OpenBao unseal material) currently share the same `runs-on` label. A poisoned PR job that lands on the same host before a production deploy could exfiltrate secrets injected into the next job's environment.

**Accepted for the initial migration.** Mitigations in place today:

- Protected GitHub Environments gate production jobs (ADR080 findings 1/2/4).
- `if: github.ref == 'refs/heads/main'` ref guards on every credentialed cluster/deploy/restore job.
- `infra.yml` keeps credentialed `terraform` off `pull_request` events.

**Follow-up (not blocking):** split runner labels — e.g. `[self-hosted, Linux, ARM64, ci]` for tests vs `[self-hosted, Linux, ARM64, production]` for credentialed deploys — and register hosts into only one pool.

## Finding 4 (critical) — runner compromise blast radius

A compromised self-hosted runner process can read every secret GitHub injects into jobs on that host, including `BAO_ROOT_TOKEN`, Shamir unseal keys, `BEX_SSH_PRIVATE_KEY`, `HCLOUD_TOKEN`, Terraform remote-state credentials, and the auto `GITHUB_TOKEN` with `contents: write` during deploy. That is strictly larger blast radius than compromising a single ephemeral GitHub-hosted VM, which disappears after the job.

**Accepted.** The trust anchor moves from "GitHub's VM hygiene" to "operator runner-host hygiene + repository access control." ADR080's environment gates add human approval before production jobs start but do not shrink what a compromised runner can read once the job runs. This is the same custody model as running `deploy.yml` logic on an operator-managed bastion — which is what self-hosted CI effectively is.

## Finding 5 (medium) — cache and Docker state poisoning

`backend-test` uses Docker service containers and a sibling `docker run` for OpenFGA; `deploy` uses Buildx. Prior jobs can leave images, volumes, or modified `/usr/local/bin` tools (several workflows `sudo install` pinned CLIs).

**Accepted.** Workflows pin third-party binaries by version + SHA-256 where the repository already established that pattern (`gitleaks`, `yq`, `clusterctl`). No additional runner scrub step is added in YAML; operators should treat runner disks as sensitive and periodically reprovision.

## Finding 6 (low) — ARM64 runner fleet

All workflows target `[self-hosted, Linux, ARM64]`. Jobs queue until an operator-registered ARM64 runner with Docker is online.

**Accepted.**

## Finding 7 (low) — onbex.co PSL (accepted, fifteenth report)

Unchanged standing residual from ADR080 #8. `.pm/DO_NOT_DO.md` `#PSL` holds the decision.

## Operator obligations (load-bearing)

These are required for the accepted dispositions above to be meaningful:

1. Register self-hosted runners only from hosts the operator controls; limit runner registration to org/repo admins.
2. Block fork PR workflows from self-hosted runners (repository Actions settings).
3. Keep ADR080's protected environments (`production-deploy`, `production-cluster`, `production-restore`, `production-infra`, `production-snapshot`, `production-release`) configured with required reviewers.
4. Maintain Docker + `sudo` on ARM64 runners.
5. Patch runner OS and `actions-runner` agent on a schedule; reprovision hosts after any suspected compromise.

## Guidance for future security scans and agents

**Do not remediate findings 1–5 by reverting `runs-on` to GitHub-hosted `ubuntu-latest`, `ubuntu-24.04-arm`, or any other GitHub-hosted label.** That is the wrong fix and CI will reject it (`scripts/github-actions-validate.sh`, `.pm/DO_NOT_DO.md` `#CI-RUNNERS`).

When a scanner or agent reports:

- "CI should use ephemeral GitHub-hosted VMs for isolation"
- "Production secrets should not run on the same runner pool as PR jobs"
- "Self-hosted runners increase compromise blast radius"

…the correct disposition is **reconfirm accepted residual** (this ADR) and point to the legitimate hardening paths below — not a workflow revert. The onbex.co PSL item (finding 7) follows the same pattern as ADR080 #8 / `.pm/DO_NOT_DO.md` `#PSL`.

**Legitimate follow-up remediations** (optional, not blocking):

- Split runner labels (`ci` vs `production`) and register hosts into one pool only.
- Ephemeral self-hosted runners (ARC / VM-per-job).
- ADR080 finding 2: secretless image build separated from credentialed deploy.
- Optional Tailscale/WireGuard network second layer ([ADR019](ADR019-infra-credentials.md) §Decision 5).

## Follow-ups

- Split runner pools by trust level (`ci` vs `production` labels) — reduces finding 3 without returning to GitHub-hosted.
- Ephemeral self-hosted runners (ARC / VM-per-job) — reduces finding 1/4 without returning to GitHub-hosted.
- ADR080 finding 2 follow-up: secretless image build separated from credentialed deploy — still open; more valuable on self-hosted than on GitHub-hosted.
- Optional Tailscale/WireGuard network second layer with tailnet-joined runners — [ADR019](ADR019-infra-credentials.md) §Decision 5; does not replace findings 1–4, complements SSH/kube exposure policy.
- onbex.co PSL submission (finding 7): `.pm/w1/050.md` — do not unset `BEX_BASE_DOMAIN` (`.pm/DO_NOT_DO.md` `#PSL`).

# ADR083: Security review round 20 — self-hosted GitHub Actions disposition

- **Status**: Accepted (2026-08-23); pool routing implemented (2026-09-02); host separation rejected — label-level split final (2026-09-07, `.pm/DO_NOT_DO.md` `#RUNNER-HOSTS`)
- **Trigger**: operator decision to migrate every workflow from GitHub-hosted (`ubuntu-latest` / `ubuntu-24.04-arm`) to self-hosted runners, now split between the `bex-ci` and `bex-production` trust-pool runner labels
- **Lineage**: twentieth pass in the ADR028 → … → ADR080 lineage; re-confirms ADR080 residuals (onbex.co PSL, secretless-build split) unchanged

## Summary

All repository workflows target self-hosted runners. That trades GitHub-hosted runners' per-job ephemeral VMs for persistent hosts the operator owns — a deliberate custody shift, not a code defect. Since 2026-09-02, every runner-backed job also carries one of two trust-pool runner labels: PR-capable and credential-free work uses `bex-ci`; production credentials and write-capable tokens use `bex-production`. Workflows select **labels**; runner registration also assigns existing org groups. Live registrations on 2026-09-05 confirm groups `bex-ci` (3) and `bex-production` (4), contradicting the earlier free-plan explanation. The actual routing defect was missing pool labels in the runner Compose configuration. The repair makes those labels durable across ephemeral registrations and separates package caches. Both pools share the operator's single Docker Desktop Linux host **by explicit user decision** (2026-09-07, `.pm/DO_NOT_DO.md` `#RUNNER-HOSTS`): host-level isolation was evaluated in w2/m88 and rejected as a hard single-machine constraint, so the label-level pool split (verified live, finding 3) is the accepted mechanism.

The remaining persistence risks are **accepted** for bex.co production CI, with the mitigations and operator obligations listed. They do not reopen the removed static-CIDR firewall (`.pm/DO_NOT_DO.md`); a Tailscale/WireGuard second layer remains optional follow-up, and self-hosted runners are now the documented path to it ([ADR019-infra-credentials.md](ADR019-infra-credentials.md) §Decision 5).

| # | Risk | Severity | Disposition |
| --- | --- | --- | --- |
| 1 | Self-hosted runners are persistent — job isolation is weaker than GitHub-hosted ephemeral VMs | high | **Accepted residual** — operator hardens hosts; ephemeral-runner follow-up optional |
| 2 | Untrusted `pull_request` jobs can execute repository-controlled code on the runner fleet | high | **Accepted residual** — fork PRs must stay off self-hosted; same-repo PRs rely on branch protection + review |
| 3 | Production credentialed jobs (`deploy`, `infra`, `app-cluster`, `snapshot`, restore drills, live cluster probes, `cli-release`) share hosts with lower-trust jobs | high | **Label-level split implemented; host separation rejected** — workflow labels and durable registration route jobs into separate pools with fail-closed validation, verified by green runs on both pools (w2/m88, 2026-09-07). Host-level separation and the associated credential rotation were rejected by user decision the same date (`.pm/DO_NOT_DO.md` `#RUNNER-HOSTS`); the shared-host residual is **accepted** |
| 4 | Runner host compromise equals platform takeover (HCLOUD, SSH admin.conf, OpenBao root + Shamir quorum, auth bootstrap secrets, `contents: write` git push) | critical | **Accepted residual** — bounded by ADR080 environment gates + main-ref guards; host custody is the new trust anchor |
| 5 | Docker layer / toolchain cache poisoning across sequential jobs | medium | **Accepted residual** — `docker/setup-buildx-action` + pinned downloads; no shared cache hardening in YAML |
| 6 | ARM64 runner fleet required | low | **Accepted residual** — operator provisions Linux ARM64 self-hosted runners with Docker |
| 7 | onbex.co PSL cross-tenant cookie poisoning | low | **Accepted residual** (fifteenth report) — unchanged from ADR080 #8; `.pm/DO_NOT_DO.md` `#PSL` |

## Context — what changed

Every runner-backed job under `.github/workflows/` and `lego/operator/.github/workflows/` now uses one of these forms:

```yaml
runs-on: [self-hosted, Linux, ARM64, bex-ci]
```

```yaml
runs-on: [self-hosted, Linux, ARM64, bex-production]
```

Both pools retain the operator's ARM64 constraint. Docker Buildx still emits `linux/amd64` platform images for production deploys.

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

**Reconfirmed 2026-09-02** (codex-security re-flag of `backend-test.yml`'s same-repo PR job on `bex-ci`). Disposition unchanged: **accepted residual**, `.pm/DO_NOT_DO.md` `#CI-RUNNERS`. Reverting `runs-on` to a GitHub-hosted label is the rejected remediation and `scripts/github-actions-validate.sh` fails closed on it. What bounds the residual is source-enforced and verified green this date: every `pull_request`-reachable job proves `github.event.pull_request.head.repo.full_name == github.repository` (public-fork isolation, check 7), never selects `bex-production`, and carries no repository/environment secret or write token (credential boundary, check 6) — so workflow-level routing excludes production credentials from CI jobs; host-level isolation was rejected 2026-09-07 (`#RUNNER-HOSTS`) and the within-host residual is accepted. The remaining within-`bex-ci` persistence (leftover files, cache/Docker state) stays operator runner-hygiene per Finding 5. The only source-side hardening that would shrink it further is ephemeral one-job runners (ARC / VM-per-job), which remains the standing optional follow-up below — not a workflow edit.

## Finding 3 (high) — shared runner pool across trust levels

Low-trust jobs (PR tests) and high-trust jobs (`build-and-deploy` with OpenBao unseal material) originally shared the same `runs-on` label. A poisoned PR job that landed on the same host before a production deploy could exfiltrate secrets injected into the next job's environment.

**Routing implemented; host separation rejected — residual accepted.** Every runner-backed workflow job carries an explicit trust-pool runner label:

- `bex-ci` carries PR-capable tests, validation, and other read-only jobs.
- `bex-production` carries repository/environment secrets, write-capable tokens, release authorization, cluster access, deploys, infrastructure, snapshots, and restore drills.
- A `bex-production` job in a mixed-event workflow must reject `pull_request` before scheduling; `infra.yml`'s `terraform` job is the canonical example.
- `scripts/github-actions-validate.sh` rejects the bare shared-pool label set, unapproved pool labels, the noncanonical runner-group mapping syntax, GitHub-hosted runners, credentials on `bex-ci`, and PR-reachable `bex-production` jobs. Its self-test proves each red and green case.

Each runner carries exactly one pool label — the sequence, API commands, and verification are in `docs/runbooks/runner-pool-relabel.md`. Until a pool has a labeled runner, its jobs intentionally remain queued. Groups exist, but are not labels. The original one-pool-per-physical-host requirement was **waived by user decision 2026-09-07** (`.pm/DO_NOT_DO.md` `#RUNNER-HOSTS`): both pools run on the operator's single Mac as a hard constraint, and the residual — a poisoned `bex-ci` runner sharing the kernel and Docker host with `bex-production` runners — is accepted. Green-run evidence for both pools on this topology is recorded in `.pm/w2/done/m88/evidence/2026-09-07-fleet-recovery.md`.

## Finding 4 (critical) — runner compromise blast radius

A compromised `bex-production` runner process can read every secret GitHub injects into jobs on that host, including `BAO_ROOT_TOKEN`, Shamir unseal keys, `BEX_SSH_PRIVATE_KEY`, `HCLOUD_TOKEN`, Terraform remote-state credentials, and the auto `GITHUB_TOKEN` with `contents: write` during deploy. That remains a larger blast radius than compromising a single ephemeral GitHub-hosted VM, which disappears after the job, and PR code shares the underlying Docker host permanently — m88's host separation was rejected by user decision 2026-09-07 (`.pm/DO_NOT_DO.md` `#RUNNER-HOSTS`), so this residual is bounded by the workflow-side controls only (environment gates, main-ref guards).

**Accepted.** The trust anchor moves from "GitHub's VM hygiene" to "operator runner-host hygiene + repository access control." ADR080's environment gates add human approval before production jobs start but do not shrink what a compromised runner can read once the job runs. This is the same custody model as running `deploy.yml` logic on an operator-managed bastion — which is what self-hosted CI effectively is.

## Finding 5 (medium) — cache and Docker state poisoning

`backend-test` uses Docker service containers and a sibling `docker run` for OpenFGA; `deploy` uses Buildx. Prior jobs within the same trust class can leave images, volumes, or tool state.

**Accepted.** Workflows pin third-party binaries by version + SHA-256 where the repository already established that pattern (`gitleaks`, `yq`, `clusterctl`). No additional runner scrub step is added in YAML; operators should treat runner disks as sensitive and periodically reprovision.

## Finding 6 (low) — ARM64 runner fleet

All jobs target the `[self-hosted, Linux, ARM64]` labels plus their `bex-ci` or `bex-production` pool label. Jobs queue until an operator-registered ARM64 runner carrying the selected pool label is online.

**Accepted.**

## Finding 7 (low) — onbex.co PSL (accepted, fifteenth report)

Unchanged standing residual from ADR080 #8. `.pm/DO_NOT_DO.md` `#PSL` holds the decision.

## Operator obligations (load-bearing)

These are required for the accepted dispositions above to be meaningful (items 1–2 are partially waived by the 2026-09-07 `#RUNNER-HOSTS` decision; 3–7 remain required):

1. Label each self-hosted runner with exactly one of `bex-ci` / `bex-production` (org Settings → Actions → Runners, or the runner-labels API — `docs/runbooks/runner-pool-relabel.md`). The one-pool-per-machine clause is waived: both pools share the operator's single Mac by decision (`.pm/DO_NOT_DO.md` `#RUNNER-HOSTS`).
2. Waived (`#RUNNER-HOSTS`): the production-host reprovision and shared-runner credential rotation are not scheduled — the shared-runner exposure window is accepted.
3. Register self-hosted runners only from hosts the operator controls; limit runner registration to organization administrators.
4. Block fork PR workflows from self-hosted runners (repository Actions settings).
5. Keep ADR080's protected environments (`production-deploy`, `production-cluster`, `production-restore`, `production-infra`, `production-snapshot`, `production-release`) configured with required reviewers.
6. Maintain Docker on ARM64 runners without granting the runner account `sudo`.
7. Patch the runner OS and `actions-runner` agent on a schedule; reprovision hosts after any suspected compromise.

## Guidance for future security scans and agents

**Do not remediate findings 1–5 by reverting `runs-on` to GitHub-hosted `ubuntu-latest`, `ubuntu-24.04-arm`, or any other GitHub-hosted label.** That is the wrong fix and CI will reject it (`scripts/github-actions-validate.sh`, `.pm/DO_NOT_DO.md` `#CI-RUNNERS`).

When a scanner or agent reports:

- "CI should use ephemeral GitHub-hosted VMs for isolation"
- "Self-hosted runners increase compromise blast radius"

…the correct disposition is **reconfirm accepted residual** (this ADR) and point to the legitimate hardening paths below — not a workflow revert. Production/PR workflow routing is guarded by `scripts/github-actions-validate.sh`; host separation was rejected by user decision (`.pm/DO_NOT_DO.md` `#RUNNER-HOSTS`) — do not re-file it. The onbex.co PSL item (finding 7) follows the same pattern as ADR080 #8 / `.pm/DO_NOT_DO.md` `#PSL`.

**Legitimate follow-up remediations** (optional, not blocking):

- Ephemeral self-hosted runners (ARC / VM-per-job).
- ADR080 finding 2: secretless image build separated from credentialed deploy.
- Optional Tailscale/WireGuard network second layer ([ADR019](ADR019-infra-credentials.md) §Decision 5).

## Follow-ups

- w2/m88 — **closed 2026-09-07**: label-level split verified live on both pools (evidence: `.pm/w2/done/m88/evidence/2026-09-07-fleet-recovery.md`); host separation + credential rotation rejected by user decision (`.pm/DO_NOT_DO.md` `#RUNNER-HOSTS`). Finding 3's disposition is final: label-level split only.

- Ephemeral self-hosted runners (ARC / VM-per-job) — reduces finding 1/4 without returning to GitHub-hosted; must stay on the single operator Mac per `#RUNNER-HOSTS`.
- ADR080 finding 2 follow-up: secretless image build separated from credentialed deploy — still open; more valuable on self-hosted than on GitHub-hosted.
- Optional Tailscale/WireGuard network second layer with tailnet-joined runners — [ADR019](ADR019-infra-credentials.md) §Decision 5; does not replace findings 1–4, complements SSH/kube exposure policy.
- onbex.co PSL submission (finding 7): `.pm/w1/050.md` — do not unset `BEX_BASE_DOMAIN` (`.pm/DO_NOT_DO.md` `#PSL`).

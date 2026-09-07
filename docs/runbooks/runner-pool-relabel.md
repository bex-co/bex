# Runbook: split the self-hosted runner fleet into `bex-ci` / `bex-production` pools

Closes ADR083 finding 3 (high): PR-capable test jobs and production-credentialed deploy jobs must never share a runner host. The repo side (workflow `runs-on` labels + `scripts/github-actions-validate.sh` enforcement, `w2/m88`) lands fail-safe — until each host carries its pool label, jobs **queue** instead of running on the wrong host. This runbook is the operator-side half: labeling each physical runner into exactly one pool.

## Mechanism: labels, not runner groups

The split is expressed as a fourth runner label — `runs-on: [self-hosted, Linux, ARM64, bex-ci]` or `runs-on: [self-hosted, Linux, ARM64, bex-production]`.

Workflows standardize on pool labels; the existing organization groups are separate registration metadata. Live inspection on 2026-09-05 confirmed `bex-ci` (group ID 3) and `bex-production` (group ID 4). The earlier claim that this organization cannot use groups was incorrect. The old scheduling failures alone did not prove a subscription limitation.

The runner source is `bex-co/block-eden-mono`, `projects/github-runner/`. Its Compose configuration originally set `RUNNER_GROUP` but omitted the matching `RUNNER_LABELS` entry. All ten runners were healthy but label-addressed jobs queued. Ephemeral runners must receive both settings on every registration; a one-time API relabel is lost after their next job.

The 2026-09-05 repair adds each service's matching pool label and separates its npm/yarn/pnpm cache volumes. Both pools currently share Docker Desktop's Linux host, so this is a scheduling repair, **not completion of m88 host isolation**. Privileged DinD containers share that host kernel. Keep m88 open until production has a separate host and the required credential rotation and live evidence hold.

## Pool assignment (the trust classification)

Classified by what each job can touch — secrets, `environment:` gates, and write-capable tokens — not by name. `scripts/github-actions-validate.sh` re-derives the credential side mechanically on every run.

**`bex-production` (12 jobs)** — repository/environment secrets, write tokens, cluster access:

| workflow | job(s) | evidence |
| --- | --- | --- |
| `deploy.yml` | `build-and-deploy` | `environment: production-deploy`; OpenBao unseal keys, TLS certs, SSH + HCLOUD tokens |
| `app-cluster.yml` | cluster apply | `environment` gate; HCLOUD + SSH admin.conf |
| `infra.yml` | `terraform` | `environment: production-infra`; Terraform state credentials |
| `snapshot.yml` | snapshot bake | `environment: production-snapshot`; `HCLOUD_TOKEN` |
| `openbao-restore-drill.yml` | drill | `environment: production-restore`; unseal keys + root token |
| `keyvalue-restore-drill.yml` | drill | restore credentials + age key |
| `ssh-edge-liveness.yml` | both jobs | `HCLOUD_TOKEN`, `BEX_SSH_PRIVATE_KEY` |
| `cli-release.yml` | both jobs | `environment: production-release`; tap push key; release write token |
| `cli-release-staleness.yml` | `staleness` | `issues: write` token |
| `build-toolchain-freshness.yml` | `freshness` | `issues: write` token |

**`bex-ci` (21 jobs)** — PR-capable and credential-free: the four reusable test workflows (`backend-test`, `dashboard-test`, `operator-test`, `opensandbox-controller-test`), `cli-test`, `go-lint`, `govulncheck`, `docs`, `scripts`, `gitops`, `clusterapi-validate`, `mobile-test`, `render-schema-drift` (2), `egress-meter-live` (kind-isolated, read-only), `infra.yml`'s credential-less `validate`, `deploy.yml`'s `check-supersession` + `secret-scan` (read-only over repo content), and the three `lego/operator` workflows (`test`, `test-e2e`, `lint`).

## Re-label sequence

Ordering rule: **production first**. A `bex-ci` job can never land on a production host (production hosts never receive the `bex-ci` label), so there is no unsafe window; labeling production first unblocks main-branch deploys before PR traffic resumes.

1. **Choose the production host(s):** the machine(s) already entrusted with deploy credentials — the hosts that ran `build-and-deploy` before the split. Every other host becomes `bex-ci`. One machine must never carry both labels (ADR083 operator obligation 1), and per obligation 2, rotate credentials that previously landed on shared runners before first production-pool use.
2. **Configure durable registration** in the runner source. Production must use `RUNNER_GROUP=bex-production` and include `bex-production` in `RUNNER_LABELS`; CI must use `RUNNER_GROUP=bex-ci` and include `bex-ci`. Never include both pool labels on one runner. Preserve the existing `self-hosted`, `Linux`, `ARM64`, and organization compatibility labels.
3. **Apply each service on its intended host** after checking for active jobs: `docker compose up -d --build runner-bex-production` on production, `docker compose up -d --build runner-bex-ci` on CI. These commands also start the emulation prerequisite. Never print `docker compose config` with real credentials; the configuration test uses a dummy token.
4. **Verify:** push (or re-run a queued run) and confirm placement per job:

   ```sh
   gh api repos/bex-co/bex/actions/runs/<run-id>/jobs \
     --jq '.jobs[] | [.name, .conclusion, .runner_name, (.labels|join(","))] | @tsv'
   ```

   Done when one CI-pool run (e.g. `test (backend)`) and one production-pool run (a real `deploy (bex via Argo)`) are green **on the intended hosts** — that is `w2/m88` t004's evidence and completes ADR083 finding 3.

## Failure modes

- **Jobs queue forever** → a pool has no host carrying that exact label (or a label typo). Compare `gh api orgs/bex-co/actions/runners` labels against the two canonical spellings. Queuing is the designed fail-safe, not an error.
- **Jobs fail in seconds with zero steps** → inspect GitHub annotations and group/repository access settings. Do not infer a plan limitation from this symptom alone; the validator still requires the canonical pool-label form.

## Remote Go cache failures

On 2026-09-05, mobile job `101379637759` attempted to restore a 4.45 GB setup-go cache; after roughly ten minutes Azure rejected the signed request. The setup action remained active after reporting the failure. Operator and CLI tests passed but their cache-save post steps also occupied runners for many minutes. This is the same failure class already documented in `go-lint.yml`.

Backend, mobile, operator, CLI-test and edge-liveness workflows disable the setup-go remote cache. Go still uses its runner-local module and build caches; no test command, failure handling, or production gate is removed. Changes take effect on newly dispatched workflow runs after shipping, not on active jobs.

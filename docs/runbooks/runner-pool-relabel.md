# Runbook: split the self-hosted runner fleet into `bex-ci` / `bex-production` pools

Closes ADR083 finding 3 (high): PR-capable test jobs and production-credentialed deploy jobs must never share a runner host. The repo side (workflow `runs-on` labels + `scripts/github-actions-validate.sh` enforcement, `w2/m88`) lands fail-safe — until each host carries its pool label, jobs **queue** instead of running on the wrong host. This runbook is the operator-side half: labeling each physical runner into exactly one pool.

## Mechanism: labels, not runner groups

The split is expressed as a fourth runner label — `runs-on: [self-hosted, Linux, ARM64, bex-ci]` or `runs-on: [self-hosted, Linux, ARM64, bex-production]`.

**Do not use GitHub runner groups.** They are a paid-plan (Team/Enterprise) feature; the bex-co organization is on the free plan, where only the built-in `default` group exists. The first cut of this split (2026-09-02, `f9294206`) addressed jobs to `runs-on.group: bex-ci|bex-production` and every job in every workflow failed **within seconds, with zero steps executed and no runner group assigned** (deploy runs 33602459228 → 33662397423) — a group-addressed job fails instantly instead of queuing, so the whole CI matrix went red on every push. An unmatched _label_, by contrast, queues: safe, visible, recoverable. `github-actions-validate.sh` now rejects the group syntax outright.

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
2. **Add `bex-production`** to the chosen host(s). Either:

   - **UI:** org **Settings → Actions → Runners →** runner **→ ⚙ → Edit labels** → add `bex-production`; or
   - **API** (needs a token with `admin:org`, e.g. after `gh auth refresh -h github.com -s admin:org`):

     ```sh
     gh api orgs/bex-co/actions/runners \
       --jq '.runners[] | [.id, .name, .status, (.labels|map(.name)|join(","))] | @tsv'
     gh api -X POST orgs/bex-co/actions/runners/<id>/labels -f 'labels[]=bex-production'
     ```

   - **On-host** re-registration also works: `./config.sh ... --labels bex-production` (keeps the implicit `self-hosted`/`Linux`/`ARM64`).

3. **Add `bex-ci`** to every remaining host the same way.
4. **Verify:** push (or re-run a queued run) and confirm placement per job:

   ```sh
   gh run view <run-id> --json jobs \
     --jq '.jobs[] | [.name, .conclusion, .runner_name, (.labels|join(","))] | @tsv'
   ```

   Done when one CI-pool run (e.g. `test (backend)`) and one production-pool run (a real `deploy (bex via Argo)`) are green **on the intended hosts** — that is `w2/m88` t004's evidence and completes ADR083 finding 3.

## Failure modes

- **Jobs queue forever** → a pool has no host carrying that exact label (or a label typo). Compare `gh api orgs/bex-co/actions/runners` labels against the two canonical spellings. Queuing is the designed fail-safe, not an error.
- **Jobs fail in seconds with zero steps** → someone reintroduced the runner-group syntax. `scripts/github-actions-validate.sh` fails closed on it; see the mechanism section above.

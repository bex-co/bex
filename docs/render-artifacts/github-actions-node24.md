# GitHub Actions Node 24 migration evidence

**Observed:** 2026-07-28 UTC

**Scope:** every external `uses:` reference under `.github/workflows/`, including nested actions used by composite security steps. Reusable local workflows are not third-party actions.

GitHub moved hosted runners to Node 24 by default on 2026-06-16 and plans to remove Node 20 in fall 2026. The repository now uses 36 direct invocations across 14 distinct external action refs; 11 distinct refs declared `runs.using: node20` at their old pins.

## Upgrades

| Action | Old | Current | Compatibility review |
| --- | --- | --- | --- |
| `actions/checkout` | v4, Node 20 | v7, Node 24 | v7 is an ESM/dependency update over v6. Existing default checkouts and `fetch-depth: 0` remain supported; the deploy write-back runs Git directly on the hosted runner, so v6's separate credential-file behavior needs no workflow change. |
| `actions/setup-go` | v5, Node 20 | v7, Node 24 | v6 added Go `toolchain` handling and changed its default cache key; maintained jobs already provide `go-version-file`, and cache-sensitive jobs provide `cache-dependency-path`. v7 is an ESM/dependency update. |
| `actions/setup-node` | v4, Node 20 | v7, Node 24 | Existing explicit Node versions remain supported. Yarn caching stays in the separately keyed `actions/cache` steps; v6's automatic-cache restriction to npm therefore does not alter the jobs. v7 is an ESM/dependency update. |
| `actions/cache` | v4, Node 20 | v6, Node 24 | `path`, `key`, and `restore-keys` are unchanged. The repository was already on the cache-service-v2 generation introduced by v4; v6 is the maintained ESM release. |
| `docker/setup-buildx-action` | v3, Node 20 | v4, Node 24 | v4 removes deprecated inputs/outputs; this repository passes none of them. |
| `docker/login-action` | v3, Node 20 | v4, Node 24 | Registry, username, and password inputs are unchanged. Permissions remain `packages: write` only in the image-publish job. |
| `docker/build-push-action` | v6, Node 20 | v7, Node 24 | v7 removes two deprecated summary environment variables and legacy summary-export support; neither is used. Context, platforms, push, tags, build args, and digest outputs remain supported. |
| `hashicorp/setup-terraform` | v3, Node 20 | v4, Node 24 | v4's published breaking change is the runtime move to Node 24. The pinned `terraform_version` input and wrapper behavior are unchanged. |
| `azure/setup-helm` | v4, Node 20 | v5, Node 24 | The pinned Helm `version` input is unchanged. |
| `azure/setup-kubectl` | v4, Node 20 | v5, Node 24 | The pinned kubectl `version` input is unchanged; v5 also resolves major/minor requests to a patch, which is irrelevant to the repository's full version pin. |
| `gitleaks/gitleaks-action` | v2, Node 20 | removed; official Gitleaks CLI v8.30.1 | The maintained action requires a separate license for organization repositories and the repository had none, so its successful warn-first job only emitted a setup annotation and did not scan. The deploy workflow now checksum-verifies the official CLI archive and scans the pushed commit range directly; warn-first policy and workflow permissions are unchanged. |

The docs-format job's explicitly installed Node runtime also moved from 20 to 24. Dashboard tests intentionally retain Node 22 because that is application-toolchain selection, not an action implementation runtime, and Node 22 remains supported.

## Reviewed unchanged actions

| Action | Runtime evidence | Decision |
| --- | --- | --- |
| `hashicorp/setup-packer@v3` | `action.yml` declares Node 24. | Keep the maintained major and existing Packer version input. |
| `anchore/sbom-action/download-syft@v0` | The sub-action declares Node 24. | Keep the project's maintained v0 major and current Syft download contract. |
| `aquasecurity/trivy-action@v0.36.0` | Composite action; its pinned setup/cache/checkout actions are composite or Node 24. | Keep the exact current release used by all four scan steps. |
| `sigstore/cosign-installer@v3` | Composite shell action with no JavaScript runtime. | Keep the current contract; a Cosign major upgrade is unrelated to Node 20 retirement. |

## Primary evidence

- [GitHub's Node 20 deprecation notice](https://github.blog/changelog/2025-09-19-deprecation-of-node-20-on-github-actions-runners/)
- [Checkout releases](https://github.com/actions/checkout/releases), [setup-go releases](https://github.com/actions/setup-go/releases), [setup-node releases](https://github.com/actions/setup-node/releases), and [cache releases](https://github.com/actions/cache/releases)
- [Docker setup-buildx v4](https://github.com/docker/setup-buildx-action/releases/tag/v4.0.0), [login v4](https://github.com/docker/login-action/releases/tag/v4.0.0), and [build-push v7](https://github.com/docker/build-push-action/releases/tag/v7.0.0)
- [HashiCorp setup-terraform v4](https://github.com/hashicorp/setup-terraform/releases/tag/v4.0.0), [Azure setup-helm v5](https://github.com/Azure/setup-helm/releases/tag/v5.0.0), and [Azure setup-kubectl v5](https://github.com/Azure/setup-kubectl/releases/tag/v5.0.0)
- [Gitleaks v8.30.1](https://github.com/gitleaks/gitleaks/releases/tag/v8.30.1) and the [Gitleaks Action organization-license requirement](https://github.com/gitleaks/gitleaks-action#readme)

`scripts/github-actions-validate.sh` freezes this reviewed set. Any new action or major now fails the scripts workflow until its runtime, release notes, inputs, permissions, and nested composite actions are reviewed and the inventory is deliberately updated.

**2026-08-23 (ADR083):** all workflows now run on self-hosted ARM64 runners (`[self-hosted, Linux, ARM64]`), not GitHub-hosted `ubuntu-*`. The validator also enforces that custody (`.pm/DO_NOT_DO.md` `#CI-RUNNERS`); reverting to `ubuntu-latest` is a rejected security-scan remediation.

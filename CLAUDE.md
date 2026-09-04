# CLAUDE.md

bex is the open-source Render alternative — AI-native ([ADR008](docs/ADR008-vision.md)). A Go Kubernetes operator reconciles `App` CRs (`app.bex.co/v1alpha1`, `bex-system`) into running services. All Go lives in **`lego/`** (Latin _legō_, "I assemble") — one image, four workspace modules: **types/** (CRD contract, leaf), **operator/** (mechanism, DB-free manager), **backend/** (bex-api on :8090 + SSH gateway), **cli/** (Render CLI launcher, `bex-cli/v*` train). `operator → types ← backend`; `cli` imports none.

## Repo map

- `lego/` — all Go, self-contained (`go.work`, `Dockerfile`; context `lego/`). [README](lego/README.md), [workspace rules](lego/CLAUDE.md).
  - `lego/types/` — `App`/`Database` CRD types
  - `lego/operator/` — manager → Deployment/Service/Ingress; owns `config/` + codegen. [operator guide](lego/operator/CLAUDE.md)
  - `lego/backend/` — bex-api (REST/GraphQL/MCP + OpenFGA) + ssh-gateway. [backend guide](lego/backend/CLAUDE.md)
  - `lego/cli/` — `bex` CLI launcher (pinned `render-oss/cli`). [README](lego/cli/README.md)
- `dashboard/` — TanStack Start + Apollo + shadcn, client of bex-api GraphQL. [dashboard guide](dashboard/CLAUDE.md)
- `infra/` — day-0: Terraform + Cluster API (`local-capd` ⇄ `hetzner-caph`)
- `deploy/gitops/` — day-1: Argo CD platform infra (not user deploys)
- `examples/` — `whoami-app.yaml`, `hello-go/`
- `scripts/` — cluster helpers (`mock-cluster.sh`, `app-apply.sh`, `deploy-sample.sh`)
- `docs/` — one file per topic. Full catalog in [docs/CLAUDE.md](docs/CLAUDE.md)
- `.pm/` — internal PM board (may be stale). Conventions in [.pm/CLAUDE.md](.pm/CLAUDE.md)

## Commands

All `make` targets live in **`lego/operator/`**; see [lego/CLAUDE.md](lego/CLAUDE.md) for workspace Go-version split + codegen. CI gates:

- `make test` (operator, from `lego/operator/`) — CRD/RBAC codegen + envtest
- `cd lego/backend && go test ./...` — backend (real Postgres + OpenFGA in CI)
- `make lint` (all four modules) — golangci-lint + whole-program dead-code analysis; depguard guards the `id` convention
- `cd lego/cli && go test ./...` — CLI launcher

All three platform suites + `dashboard/yarn test` must pass before `deploy.yml` builds.

## Local cluster workflow

- `bash scripts/mock-cluster.sh` — kind infra + CAPI + CAPD app cluster; kubeconfig `infra/local/bex.kubeconfig` (gitignored)
- `bash scripts/mock-cluster.sh scale N` — add/remove workers
- `scripts/app-apply.sh <bex.yml>` — `render.yaml`-shaped `bex.yml` → App CR (`DRY_RUN=1` preview)
- `scripts/deploy-sample.sh` / `kubectl get apps.app.bex.co` — deploy + status

## Environment variables

Inventories live with the code (cascading):

- operator / activator / pg-sni-proxy / kv-sni-proxy / egress-meter / static-server → [lego/operator/CLAUDE.md](lego/operator/CLAUDE.md)
- bex-api / ssh-gateway → [lego/backend/CLAUDE.md](lego/backend/CLAUDE.md)
- dashboard SSR → [dashboard/CLAUDE.md](dashboard/CLAUDE.md)

## Docs — where to look

Full ADR/ledger catalog with one-line summaries: **[docs/CLAUDE.md](docs/CLAUDE.md)** (cascading — loaded only when working in `docs/`).

Key entry points:

- Vision/roadmap: [ADR008](docs/ADR008-vision.md) · Architecture: [ADR002](docs/ADR002-architecture.md) · Control plane: [ADR003](docs/ADR003-control-plane.md)
- API core + parity: [ADR006](docs/ADR006-bex-api.md) · [ADR018](docs/ADR018-render-parity.md) · [ADR049](docs/ADR049-render-yaml-parity.md)
- IDs: [ADR020](docs/ADR020-identifiers.md) · Auth: [ADR012](docs/ADR012-auth.md) · Members: [ADR024](docs/ADR024-members.md)
- Deploy/custom-domain: [ADR004](docs/ADR004-app-deployment.md) · [ADR005](docs/ADR005-custom-domain.md)
- Managed data: [ADR009](docs/ADR009-postgresql-management.md) · [ADR021](docs/ADR021-keyvalue-management.md) · [ADR029](docs/ADR029-static-sites.md)
- Billing/pricing: [ADR040](docs/ADR040-billing-metronome.md) · [ADR030](docs/ADR030-pricing.md)
- GitHub/members/infra-creds: [ADR026](docs/ADR026-github-integration.md) · [ADR019](docs/ADR019-infra-credentials.md)
- Tenant isolation/networking: [ADR043](docs/ADR043-tenant-namespace-isolation.md) (replaces ADR022 option B)
- Security review lineage: [ADR028](docs/ADR028-security-review.md) → [ADR072](docs/ADR072-security-review-round7.md) … [ADR083](docs/ADR083-security-review-round20.md) (see docs/CLAUDE.md for full chain)

## Rules

- **Never `git commit`/`push` unless user runs `/ship` (Claude) or `$ship` (Codex), or explicitly requests a `routine-*` run.** A routine request authorizes planning, fixing, verification, and invoking ship in the same run without first filing a `.pm` milestone. Honor explicit audit-only or no-ship limits; follow the ship skill’s safety rules.
- Never commit/print `.env` or `*.kubeconfig`.
- **Skill layout:** canonical `.claude/skills/<name>/SKILL.md`; `.agents/skills/<name>` is `../../.claude/skills/<name>` symlink; no `.claude/commands/`. Validate: `bash scripts/skill-layout-validate.sh`.
- **`.env.example` mirrors `.env` names** (no values). `cp .env.example .env` must never fall out of date; `scripts/gh-secrets.sh` pushes `.env` → GitHub secrets.
- Markdown CI: `npx prettier@3.4.2 --write "**/*.md"` before finishing.
- **Go ids:** mint only via `lego/backend/internal/id` (`id.New(kind)`), hyphen not underscore; boilerplate header per `lego/operator/hack/boilerplate.go.txt`. See [lego/CLAUDE.md](lego/CLAUDE.md).
- **`.pm` done:** move folder to `done/` (task `tNNN.md` → `done/tNNN.md`; milestone `mN/` → `wN/done/mN/`), leave no stub; sync status in workstream README + milestone README + frontmatter. See [.pm/CLAUDE.md](.pm/CLAUDE.md).
- **Dashboard preloading skeletons must match their post-loading contents.** A skeleton is a structural preview of the exact ready state at the same responsive breakpoint: preserve its outer bounds, padding, max-width, columns, headings/actions, tabs, and major content regions with stable heights. Do not substitute a generic list/form/detail skeleton when the destination geometry differs. Verify pending and ready states side by side at desktop and narrow-mobile widths; see [dashboard/CLAUDE.md](dashboard/CLAUDE.md#navigation-pending-states-the-white-flash-fix).
- Playwright MCP: writes to `.playwright-mcp/` (`--output-dir` in `.mcp.json`); use bare filenames for screenshots.

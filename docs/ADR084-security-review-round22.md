# ADR084: Security review round 22 — dnR05P disposition

- **Status**: Accepted (2026-08-28)
- **Scan**: codex-security `dnR05P`, repository revision `643d1869`, 17 findings (6 medium, 11 low)
- **Lineage**: twenty-second pass in the ADR028 → … → ADR083 lineage

## Summary

Each finding was retraced against the exact scanned revision. Three reportable defects are fixed: ACL-bearing environment membership teardown now preserves the administrator boundary on every call path (finding 3), the bootstrap management-cluster kubeconfig is root-only (finding 14), and repository links show their actual forge hostname in dashboard and deploy-email labels (finding 15). Five rejected findings exposed inexpensive hardening opportunities that are also applied: delivery Git cannot execute tenant hooks (finding 4), push routes are resolved and checked as same-origin (finding 8), OpenSandbox ids cannot contribute URL syntax or redirect a tenant capability (finding 11), disk restore uses a rooted filesystem API (finding 12), and env-group unlink enforces the same workspace-pair invariant as link (finding 13).

Two real risks remain explicitly accepted: the standing `onbex.co` Public Suffix blocker (finding 1, `.pm/DO_NOT_DO.md` `#PSL`) and cluster-wide interactive access held by the trusted routine-operations credential (finding 5, ADR057). The remaining seven findings do not have the attacker reachability or incremental impact claimed by the report.

| # | Finding | Report severity | Disposition |
| --- | --- | --- | --- |
| 1 | Shared tenant cookie scope on non-PSL `onbex.co` | medium | **Accepted residual** — standing `#PSL` decision; must close before open signup |
| 2 | Agent sandbox template lacks a security context | medium | **Rejected** — UID 10001 has no effective default capabilities and cannot rewrite the root-owned driver; read-only rootfs conflicts with workspace/snapshot semantics |
| 3 | Developer can tear down protected-environment ACL state | medium | **Fixed** — fresh `can_manage` at shared ACL-bearing teardown/membership seams before mutation |
| 4 | Delivery Git executes tenant repository hooks/config | medium | **Hardened** — finding lacks incremental privilege, but all delivery Git uses isolated config, disabled hooks/helpers, and `--no-verify` |
| 5 | Routine operations credential can exec into platform pods | medium | **Accepted residual** — explicit cluster-wide interactive-debugging posture from ADR057; credential possession is already trusted operator access |
| 6 | Same public App name blocks another tenant's builds | medium | **Rejected** — stored App CR names are `core.CRName(tenant, name)` and therefore globally distinct |
| 7 | Driver transcript route is unauthenticated | low | **Rejected** — Cilium admits only the SSH gateway; in-pod code already owns the emitted workspace/output |
| 8 | Push relative-route check accepts backslash authority syntax | low | **Hardened** — no attacker controls stored routes, but both browser and server guards now reject parser-confusing input |
| 9 | Driver grants lack lifetime ceiling/request binding/durable replay state | low | **Rejected** — current mint sites hardcode 15/60-second grants and no attacker-controlled restart/replay chain exists |
| 10 | Static-site budgets collide on workspace-local name | low | **Rejected** — `Site.AppID` is the tenant-prefixed App CR name, not the public service name |
| 11 | Sandbox id contributes syntax to OpenSandbox URL | low | **Hardened** — pinned routes yield no meaningful sink, but ids are path-escaped and redirects are disabled |
| 12 | Snapshot tar restore follows escaping symlinks | low | **Hardened** — tenants cannot supply archives, but extraction now uses `os.Root` and refuses links/writes outside the disk root |
| 13 | Env-group unlink omits workspace-pair check | low | **Correctness fixed** — caller already controls both resources, but mismatched workspace pairs are now refused before either mutation |
| 14 | Bootstrap k3s kubeconfig is world-readable | low | **Fixed** — explicit `--write-kubeconfig-mode 600` |
| 15 | Repository links hide an attacker-chosen forge hostname | low | **Fixed** — self-hosted forges remain supported while the hostname is visible beside owner/repo and commit SHA |
| 16 | Contributor can create a managed Postgres export | low | **Rejected** — intentionally pinned `can_operate` lifecycle action; retrieval/presigning remains `can_view_sensitive` plus fresh authorization |
| 17 | Agent can bypass the driver's pre-push credential scan | low | **Rejected** — model proxy is mandatory, the real BYO key never enters the sandbox, and writes remain confined to the session branch |

## Fixes

### Finding 3 — ACL-bearing environment teardown

`environments.Service.authorizeACLBearingMutation` now performs a fresh workspace `can_manage` check at the shared mutation seams. `clearEnvironmentMembers` covers direct environment deletion and project-delete cascades; `SetServices` and `setResourceMembers` authorize before changing service, database, key-value, or env-group membership. Tests prove a developer refusal leaves both store membership and projected resource state unchanged.

### Findings 4, 8, 11, 12, and 13 — boundary hardening

- Every driver Git invocation forces `core.hooksPath=/dev/null`, disables fsmonitor/external diff helpers, ignores system/global config, disables terminal prompts, and uses `--no-verify` for commit/push. A real executable pre-commit hook proves delivery does not run it.
- The push service worker resolves a candidate route and compares origins before navigation. The Go producer rejects backslashes, authorities, malformed request URIs, and oversized routes.
- The OpenSandbox client path-escapes every id and uses `http.ErrUseLastResponse`, so the workspace tenant key never follows an upstream redirect.
- Disk extraction uses Go's rooted filesystem handle for directory, symlink, and file writes. Absolute and `..`-escaping symlink targets fail before any later entry can traverse them.
- `UnlinkService` now checks that the authorized App and env group share one workspace before detaching the App or rewriting group links.

### Finding 14 — bootstrap kubeconfig permissions

The disposable CAPI management node now installs k3s with `--write-kubeconfig-mode 600`. Operators fetch the file as root through the existing Terraform output; no non-root local reader needs direct access.

### Finding 15 — honest repository-link labels

bex supports arbitrary self-hosted Git forges, so a fixed host allowlist would be a product regression. Instead, every recognized dashboard repo label renders `host · owner / repo`, and deploy email references render `Commit <sha> on <host>`. The HTTPS destination remains unchanged, but a co-member can see the real authority before clicking.

## Accepted residuals

- **Finding 1 / `#PSL`**: real cross-tenant cookie injection remains accepted while signup is closed. Do not unset `BEX_BASE_DOMAIN`; reopen the PSL submission before open signup.
- **Finding 5 / routine operations exec**: the scoped operator kubeconfig deliberately supports cluster-wide interactive debugging, recorded in ADR057. It is a trusted, short-lived operator credential, not a tenant role. Platform-namespace exec can be moved to break-glass later only as an explicit operations-policy change.

## Verification

Focused regression suites cover every changed subsystem. Repository-wide backend tests, operator `make test`, Go lint, dashboard tests/lint/typecheck, driver delivery tests/typecheck, Markdown formatting, and `git diff --check` pass.

The driver's unrelated `agent crash becomes a failed status instead of hanging` test currently races the ACP SDK's generic connection-close error against child stderr and fails on this host. The same failure reproduces from an archived clean `643d1869` tree; the delivery suite covering finding 4 is green. Terraform is not installed in the verification environment, so the one-token cloud-init mode change was diff-reviewed rather than passed through `terraform fmt -check`.

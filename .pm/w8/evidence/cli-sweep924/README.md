# CLI sweep 924 — forged workspace header is ignored, not honored (live, 2026-10-20)

Read-only probes. No new bug filed.

- `X-Bex-Workspace: tea-000…` (synthetic) on a list call → 200 answering the caller's **default** workspace, not an error and not the named workspace: the header cannot steer authorization scoping (consistent with sweeps 809/821/837, where the header demonstrably does not rescope REST lists or GraphQL). Workspace selection flows through validated membership (`ownerId` arguments, CLI workspace), never through an unverified header claim.

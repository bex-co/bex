---
name: rt-normalize-abstractions
metadata:
  internal: "true"
description: >-
  Audits and fixes layering violations against the workspace import DAG (operator→types←backend; cli standalone; dashboard consumes the API, never reimplements it) and adds depguard rules so violations can't return. Use when the user asks to run the rt-normalize-abstractions routine, audit module boundaries, or fix layering violations.
---

# Routine: Abstraction Police

Enforce the layering: `operator → types ← backend`; `types` imports no sibling; `cli` imports no lego sibling; the dashboard talks to the backend only through its API client and never reimplements server-authoritative logic.

## Routine ground rules

Follow the shared contract in [`../../rt-shared-contract.md`](../../rt-shared-contract.md) (execution, tidy-routine value bar, batch cleanup & ≤15-line minimum threshold, **never opening ledger-only PRs**, board isolation, freshness check, `scripts/rt-pre-pr-check.sh` pre-PR fast gate, and report format).

- **Scope:** parse `$ARGUMENTS` per Phase 1. Empty = all module boundaries.

## Phase 1 — Scope

Parse `$ARGUMENTS` as an optional module/boundary. Empty = all boundaries.

## Phase 2 — Mechanical audit

- Per Go module, diff actual imports against the law: `go list -deps ./...` or `grep -rn '"github.com/bex-co/bex/lego/'`.
- Within-module layer skips: `cmd/` reaching deep into internals instead of the intended package surface.
- Check whether `lego/backend/.golangci.yml` and `lego/operator/.golangci.yml` encode the DAG in depguard rules (backend's depguard currently guards the id convention). Missing DAG rules are themselves a finding.

## Phase 3 — Semantic audit

A reimplemented rule is a drift bug waiting or already here. For each client-side copy of a server rule, compare the two on the edge cases (limits, cooldowns, plan gates, which error wins): where the client allows what the server refuses, or refuses what the server allows, that is a user-visible bug. Prove it and fix it first; it outranks a move that only tidies the import graph.

Parallel agents compare dashboard logic against `lego/backend/internal/apps`: blueprint validation and pricing are the likely offenders. Logic that must stay server-authoritative (pricing, plan diffs, authz decisions) may not live client-side; the dashboard renders API answers.

## Phase 4 — Fix

- Wrong-direction or sideways imports → move the shared code to `lego/types` (the only module both operator and backend may import) and migrate callers.
- Dashboard reimplementations → replace with API data (extend the GraphQL/REST response if a field is missing).
- Add depguard rules encoding the DAG to the golangci configs so `make lint` catches future violations.

## Phase 5 — Verify and report

Types moves ripple: `go build ./...` and `go vet ./...` in every module that imports the moved code (this compiles their tests without running them), then the targeted tests. The new depguard rules are this routine's own output, so prove them once on the final tree. From each module that gained a rule, run `"$PWD/../operator/bin/golangci-lint" run --enable-only depguard` (from `lego/operator` itself, `bin/golangci-lint`; `make golangci-lint` there installs the pinned binary). Seed one violation it must flag, then revert it. The full `make lint` runs in CI. Dashboard: the targeted dashboard checks if touched. Report per the standard format.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.

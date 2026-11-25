---
name: rt-flatten-abstractions
metadata:
  internal: "true"
description: >-
  Finds over-engineered abstractions — single-implementation interfaces, pass-through layers, generics instantiated at one type, wrapper hooks that add nothing — and flattens them to direct code; module-boundary seams are left to rt-normalize-abstractions. Use when the user asks to run the rt-flatten-abstractions routine, remove over-engineering, or flatten needless indirection.
---

# Routine: Abstraction Flattener

Indirection has to earn its keep. Flatten what doesn't.

## Routine ground rules

Follow the shared contract in [`../../rt-shared-contract.md`](../../rt-shared-contract.md) (execution, tidy-routine value bar, batch cleanup & ≤15-line minimum threshold, **never opening ledger-only PRs**, board isolation, freshness check, `scripts/rt-pre-pr-check.sh` pre-PR fast gate, and report format).

- **Scope:** parse `$ARGUMENTS` per Phase 1. Empty = all production code in the repo per the shared contract (excluding `examples/`, `**/testdata/**`, `**/fixtures/**`, `docs/`, `.pm/`, generated code) via parallel per-area agents.

## Phase 1 — Scope

Parse `$ARGUMENTS` as an optional path/module scope. Empty = whole repo via parallel per-module agents.

## Phase 2 — Find candidates

- Go interfaces with exactly one non-test implementation (`grep -rn "^type .* interface" lego/`, then find implementers). A test fake doesn't count as a second implementation — but interfaces that exist specifically to enable envtest/controller-runtime fakes in `lego/operator` are often legitimate; skip those.
- Functions that only forward to one other function with the same arguments.
- Generic type parameters instantiated at exactly one type.
- Dashboard wrapper hooks/components adding no behavior over what they wrap.

## Phase 3 — Check recorded intent

An abstraction is worth flattening when it costs something (value bar): a change that has to be made in two places, behavior a reader cannot find without chasing the indirection, or a seam that has let two callers diverge. When two callers reach the same thing through different layers, compare what each ends up doing: a difference is a drift bug, proved with a failing test and fixed first. A one-line alias, or a wrapper nobody trips over, is not a finding.

`git log` the abstraction: added recently for a planned second implementation is different from untouched for a year. Check `docs/` ADRs and `.pm/DO_NOT_DO.md` for recorded intent. Module-boundary seams (types↔operator↔backend contracts) are `/rt-normalize-abstractions` territory — keep them even when single-impl.

## Phase 4 — Flatten

Inline the single implementation, replace interface parameters with the concrete type, delete pass-through layers, collapse single-use generics. Run the package's tests after each flatten. If a flatten forces test rewrites beyond mechanical type substitution, the abstraction was load-bearing — revert and skip with a note.

## Phase 5 — Verify and report

Targeted checks (ground rules): a flattened interface's callers are its importers, so test those packages too. Report per the standard format.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.

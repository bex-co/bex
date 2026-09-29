---
name: routine-crash-fuzzer
description: >-
  Hunts real application crashes — first by listening to bex platform service logs for panics, crash loops, and 5xx bursts, then by fuzzing untrusted-input parsers with Go native fuzzing — and lands root-cause fixes with regression tests. Use when the user asks to run the crash-fuzzer routine, hunt panics or crashes, fuzz the platform, or investigate service crashes.
---

# Routine: Crash Fuzzer

Find crashes from two evidence sources — live platform service logs first (real crashes beat hypothetical ones), then Go native fuzzing of untrusted-input parsers — and fix each at its root cause.

## Routine ground rules

- A user request to run this routine authorizes the full cycle: plan, fix, verify, then invoke `$ship` (Codex) or `/ship` (Claude) directly in the same run. Honor explicit user limits such as audit-only or no-ship.
- Keep a concise execution plan in the conversation and update it as findings are confirmed. Continue into implementation without a planning approval pause. Do not create or file a `.pm` milestone/task as a prerequisite or substitute for fixing; use the board only if the user requests it.
- After the verification phase, follow the shipping phase below without asking the user to invoke ship separately. Preserve the ship skill's branch, conflict-resolution, and safety rules.
- Fix everything you find directly in the working tree. If a fix can't be finished safely in this run, revert only your partial work for that fix and list it under **Deferred** in the report — never leave the tree half-refactored.
- Scope: parse `$ARGUMENTS` per Phase 1 — an optional path/module scope or skill-specific target. No argument = the skill's stated default (usually the whole repo via parallel agents per module: lego/types, lego/operator, lego/backend, lego/cli, dashboard).
- False-positive discipline: if a finding is intentional, load-bearing, or ambiguous, skip it with a one-line reason — don't argue it into a change.
- Every change must pass the verification gates for the modules it touched before being reported as done: operator `make test` + `make lint` (from `lego/operator/`), `cd lego/backend && go test ./...`, `cd lego/cli && go test ./...`, dashboard `yarn test` + typecheck.
- Run `npx prettier@3.4.2 --write` on any markdown touched.
- Before shipping, summarize results in commentary using: **Fixed** (file:line + one-line rationale) / **Deferred** (item + why) / **Skipped** (finding + reason) / **Gates** (commands run + outcomes).

## Phase 1 — Scope

Parse `$ARGUMENTS`:

- `logs` — run Phase 2 only.
- `blueprint` | `rest` | `graphql` | `operator` — run Phase 3 only, on that target family.
- A package path — fuzz that package only.
- Empty — full run: Phase 2, then Phase 3 in priority order.

## Phase 2 — Listen to platform service logs

1. Resolve a cluster, in priority order; never print kubeconfig contents:
   - **Production:** `BEX_SSH_KEY_PATH=~/.ssh/id_bex bash scripts/fetch-app-kubeconfig.sh /tmp/bex-app.kubeconfig`, then `export KUBECONFIG=/tmp/bex-app.kubeconfig`. Delete `/tmp/bex-app.kubeconfig` when the phase ends.
   - **Local:** `infra/local/bex.kubeconfig` if the mock cluster is up.
   - Neither reachable → note it in the report and continue to Phase 3.
2. Check restarts and events: `kubectl -n bex-system get pods` (restart counts) and `kubectl -n bex-system get events --sort-by=.lastTimestamp` (OOMKilled, CrashLoopBackOff, probe failures).
3. Sweep logs for each platform service (operator, bex-api, ssh-gateway, activator, pg-sni-proxy, kv-sni-proxy, egress-meter, static-server): `kubectl -n bex-system logs --prefix --since=48h deploy/<svc>`, plus `--previous` for any container that restarted. Grep for `panic:`, `runtime error`, `fatal error`, `goroutine `, SIGSEGV, and 5xx bursts.
4. Deduplicate into distinct crash signatures. For each: root-cause from the stack trace in the source, fix the actual defect, and add a regression test reproducing the crashing input/state. Keep every crashing input as a fuzz seed for Phase 3.

## Phase 3 — Fuzz untrusted-input parsers

Target families in priority order (the repo has no fuzz tests today — the targets you write are durable artifacts):

1. **Blueprint compiler** (highest value — untrusted render.yaml → IR → plan): `CompileBlueprintSource` in `lego/backend/internal/apps/blueprint_compiler.go`, plus the IR/plan pipeline (`blueprint_ir.go`, `blueprint_plan.go`) and `lego/backend/internal/api/render_openapi.go`.
2. **REST decoding** — stdlib `ServeMux` handlers in per-feature `rest.go` files under `lego/backend/internal/`.
3. **GraphQL resolvers** — per-feature `graphql.go` files.
4. **Operator** — CRD defaulting/validation feeding the four reconcilers in `lego/operator/internal/controller/`.

For each target:

1. Write a persistent `Fuzz*` target in a `*_fuzz_test.go` beside the code. Seed with `f.Add` calls harvested from real fixtures in the package's existing tests (`blueprint_*_test.go` is full of render.yaml strings) and any Phase 2 crashing inputs. Seeds live as `f.Add` calls so they're reviewable; NEVER commit `testdata/fuzz/` cache directories.
2. Run with a budget: `go test -run '^$' -fuzz FuzzXxx -fuzztime 90s ./path/`. Cap total fuzzing wall time at ~10 minutes across all targets.
3. For each crasher: minimize the input, root-cause the panic — fix the actual nil deref/index/overflow, never wrap in a blanket `recover` — and promote the input to a named `f.Add` seed or a plain regression `TestXxx`.

## Phase 4 — Verify and report

Run the gates for every module touched (see ground rules). Then report in the standard format, including crash signatures found in logs, fuzz targets added, and crashers fixed.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.

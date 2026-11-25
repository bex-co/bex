---
name: rt-fix-prod-crashes
metadata:
  internal: "true"
description: >-
  Hunts real application crashes — first by listening to bex platform service logs for panics, crash loops, and 5xx bursts, then by fuzzing untrusted-input parsers with Go native fuzzing — and lands root-cause fixes with regression tests. Use when the user asks to run the rt-fix-prod-crashes routine, hunt panics or crashes, fuzz the platform, or investigate service crashes.
---

# Routine: Crash Fuzzer

Find crashes from two evidence sources — live platform service logs first (real crashes beat hypothetical ones), then Go native fuzzing of untrusted-input parsers — and fix each at its root cause.

## Routine ground rules

Follow the shared contract in [`../../rt-shared-contract.md`](../../rt-shared-contract.md) (execution, anti-noise value bar, **never opening ledger-only PRs**, board isolation, freshness check, `scripts/rt-pre-pr-check.sh` pre-PR fast gate, and report format).

- **Scope:** parse `$ARGUMENTS` per Phase 1. Empty = full run: Phase 2, then Phase 3 in priority order.

## Phase 1 — Scope

Parse `$ARGUMENTS`:

- `logs` — run Phase 2 only.
- `blueprint` | `rest` | `graphql` | `operator` — run Phase 3 only, on that target family.
- A package path — fuzz that package only.
- Empty — full run: Phase 2, then Phase 3 in priority order.

## Phase 2 — Listen to platform service logs

1. Resolve a cluster, in priority order; never print kubeconfig contents:
   - **Production:** run `bash scripts/fetch-app-kubeconfig.sh /tmp/bex-app.kubeconfig` first (it loads the token from `.env` itself, the SSH key defaults to `~/.ssh/bex`), then `export KUBECONFIG=/tmp/bex-app.kubeconfig`. Delete `/tmp/bex-app.kubeconfig` when the phase ends. A missing key file under a name an older note gave is **not** a reason to skip production: the key is `~/.ssh/bex`, so try the fetch before you write "no production access".
   - **Local:** `infra/local/bex.kubeconfig` if the mock cluster is up.
   - Neither reachable → note it in the report and continue to Phase 3.
2. Check restarts and events: `kubectl -n bex-system get pods` (restart counts) and `kubectl -n bex-system get events --sort-by=.lastTimestamp` (OOMKilled, CrashLoopBackOff, probe failures).
3. Sweep logs for each platform service (operator, bex-api, ssh-gateway, activator, pg-sni-proxy, kv-sni-proxy, egress-meter, static-server): `kubectl -n bex-system logs --prefix --since=48h deploy/<svc>`, plus `--previous` for any container that restarted. Grep for `panic:`, `runtime error`, `fatal error`, `goroutine `, SIGSEGV, and 5xx bursts.
4. Deduplicate into distinct crash signatures. For each: root-cause from the stack trace in the source, fix the actual defect, and add a regression test reproducing the crashing input/state. Keep every crashing input as a fuzz seed for Phase 3.
5. Production logs carry tenant data, and this repository is public ([rule](../../../.pm/AGENTS.md#the-board-is-public)). Before a crash signature, log excerpt or crashing input goes into a seed, test, commit message, PR or `ledger.md`, replace tenant ids, service and host names and emails with synthetic values, or hash them with `scripts/lib/public_log.py`. Name only bex's own fixture workspaces and services.

## Phase 3 — Fuzz untrusted-input parsers

Target families in priority order (check existing `*_fuzz_test.go` files first so you expand uncovered untrusted-input parsers or add newly changed parsers rather than duplicating existing targets):

1. **Blueprint compiler** (highest value — untrusted render.yaml → IR → plan): `CompileBlueprintSource` in `lego/backend/internal/apps/blueprint_compiler.go`, plus the IR/plan pipeline (`blueprint_ir.go`, `blueprint_plan.go`) and `lego/backend/internal/api/render_openapi.go`.
2. **REST decoding** — stdlib `ServeMux` handlers in per-feature `rest.go` files under `lego/backend/internal/`.
3. **GraphQL resolvers** — per-feature `graphql.go` files.
4. **Operator** — CRD defaulting/validation feeding the four reconcilers in `lego/operator/internal/controller/`.

For each target:

1. Write a persistent `Fuzz*` target in a `*_fuzz_test.go` beside the code. Seed with `f.Add` calls harvested from real fixtures in the package's existing tests (`blueprint_*_test.go` is full of render.yaml strings) and any Phase 2 crashing inputs. Seeds live as `f.Add` calls so they're reviewable; NEVER commit `testdata/fuzz/` cache directories.
2. Run with a budget: `go test -run '^$' -fuzz FuzzXxx -fuzztime 90s ./path/`. Cap total fuzzing wall time at ~10 minutes across all targets.
3. For each crasher: minimize the input, root-cause the panic — fix the actual nil deref/index/overflow, never wrap in a blanket `recover` — and promote the input to a named `f.Add` seed or a plain regression `TestXxx`.

## Phase 4 — Verify and report

Run the targeted checks (see ground rules). Then report in the standard format, including crash signatures found in logs, fuzz targets added, and crashers fixed.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.

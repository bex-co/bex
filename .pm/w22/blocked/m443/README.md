# w22 · m443 — Sandbox API rate limits match Render's per-operation table

**Worker:** worker22 **Goal:** a user's sandbox calls are rate limited per operation class at Render's documented rates (create 100, terminate 1,000, exec 1,000, file transfer 1,000, reads 400, group writes 100, snapshot capture 10 per minute), per user and shared across that user's API keys, on REST, GraphQL and MCP, instead of sharing bex-api's general 500 per minute. **Status:** blocked (t001–t005 and t007–t009 done and shipped in `b7bc85e30`; t006's production run 45279460837 passed every create check, and its exec checks wait on w22/029, then t010)

## Tasks (in order)

| id   | title                                                                                                                                   | est | depends_on       |
| ---- | --------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Design record: Render's rate table in ADR104 D7 and ADR018, which route belongs to which class, and what the bucket key is — **DONE** | 40m | — |
| t002 | A per-user, per-class limiter on `core.KeyedRateLimiter`; sandbox routes stop spending the general 500 per minute — **DONE** | 60m | t001 |
| t003 | REST: the sandbox and sandbox-group routes and the run and file connect-token mints take their class's bucket — **DONE** | 50m | t002 |
| t004 | GraphQL and MCP sandbox operations use the same buckets; the 429 body and `Retry-After` match the existing limiter's — **DONE** | 40m | t002 |
| t005 | The snapshot-capture class (10 per minute) lands as a constant with tests, ready for w32/m285's capture route — **DONE** | 20m | t002 |
| t006 | Production check: a matrix leg where the 101st create in a minute answers 429 and 600 execs in a minute are all accepted                | 45m | t003, t004       |
| t007 | Render parity — **DONE** | 20m | t003, t004, t005 |
| t008 | Simplify — **DONE** | 20m | t007 |
| t009 | Test coverage — **DONE** | 30m | t007 |
| t010 | Closeout                                                                                                                                | 10m | t006, t008, t009 |

## Definition of done

- One user, using two API keys they created, gets 429 on the 101st sandbox create inside a minute; the first 100 succeed (subject to the group's concurrency limit, which is a different 429).
- The same user runs 600 execs inside a minute and none is refused by the rate limiter. Today the 501st request of any kind is refused (`BEX_RATE_LIMIT` default `500`).
- Each class in Render's table has its own bucket at Render's rate, and a call spends exactly one class. REST, GraphQL and MCP calls for the same operation spend the same bucket.
- A sandbox call no longer spends the general per-subject budget, and a non-sandbox call never spends a sandbox bucket.
- The 429 a sandbox bucket answers has the same body shape and `Retry-After` header as the existing limiter's 429.
- ADR104 D7 and ADR018 each carry the row, citing render.com/docs/sandboxes-limits with the read date; the ADR018 cells are ✅ once t006 passes.
- Every class has an automated test, and the production check is recorded in this README with the observed codes.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w22` (2026-11-16), item 1, filed by `/pmx 1,3,4,5 to w22`.
  - **Render** ([render.com/docs/sandboxes-limits](https://render.com/docs/sandboxes-limits), read 2026-11-16, public beta, "limits are subject to change"): "These limits apply per user, shared across that user's API keys. Requests over a limit fail with HTTP `429`." Requests per minute: create a sandbox 100; terminate a sandbox 1,000; execute a command 1,000; upload or download a file or directory 1,000; read sandbox and sandbox group resources 400; write sandbox group resources 100; capture a snapshot 10. Single-vendor documentation; bex has no Render account to measure it.
  - **bex today:** one general limiter for every operation. `lego/backend/cmd/api/config.go` (~line 770) reads `BEX_RATE_LIMIT` with default `500` per minute; `lego/backend/internal/api/ratelimit.go` keys a bucket by the authenticated subject and, when wired, a second aggregate bucket by workspace; `lego/backend/cmd/api/main.go` (~line 1882) builds it. `lego/backend/internal/sandbox/` has no per-operation limit. So exec, file transfer and terminate are stricter than Render (500 against 1,000) and create and group writes are looser (500 against 100).
  - Neither ADR104 nor ADR018 has a row for this table.
- **Goal linkage:** ADR008 § Render parity first; ADR104 D7 (limits) and D11 (abuse bounds).
- **Expected outcome:** an agent workload sized to Render's documented rates is not refused early on bex, and a create storm is refused where Render refuses it.
- **Why now:** w32/m285 (snapshots) is about to start and ADR104 D4 requires a capture rate limit, which t005 supplies. bex-api's limiter is in-process and single-replica today, so adding classes is simplest before it scales out.
- **Open recommendations, not user decisions** (the tasks implement them under the parity rule; t001 records them in ADR104 as recommendations):
  - Sandbox routes are counted only against Render's per-user table and leave the general 500 per minute, because otherwise 1,000 execs a minute cannot be reached.
  - The existing workspace aggregate bucket stays as an ADR104 D11 abuse bound for sandbox calls, sized above the sum a single user can reach under Render's table so it never trips first. It is a bex-owned control, not a Render fact.
  - "Per user" maps to the subject that owns the API key, so every key a user created shares the bucket. t001 confirms how the existing limiter resolves an API key to its creator.
- **Render parity included:** the limits are customer-facing on REST, GraphQL and MCP, so t007 compares each surface with Render's table.

## Outcome 2026-11-20 (/loopx w22): shipped in code, the production check waits for the deploy

- **Verdict: work.** The premise held on `26b14bb1f`: every sandbox call spent the general `BEX_RATE_LIMIT` budget and `lego/backend/internal/sandbox/` had no per-operation limit.
- **How it was built.** An earlier worker was interrupted on 2026-11-17 with the implementation uncommitted. This run rebased it over 869 commits, merged it with what main had changed since, checked every task against the code and finished the rest.
  - `8a4279185` made each member of a GraphQL batch cost a general token. The limiter's two paths now reserve through one helper, and each batch member gives its own token back when it is sandbox-only.
  - w2/m532 turned the brokered-credential routes from named 404s into seven served routes. They are classified as group resources (reads `read`, writes `group_write`), and the "not classified" exception is gone.
  - w9/m481 added `core.Base.KeyCreators`. The limiter resolves a key's creator through it instead of a lookup and cache of its own.
  - The live wrapper's two-key user is now a developer of the workspace, because since w9/m481 a key is allowed nothing its creator is not.
- **Deviations from the task files.**
  - t005: w32/m285 shipped its capture route first, so the `capture` class is charged on `POST /v1/sandboxes/{id}/snapshots`, `createSandboxSnapshot` and `create_sandbox_snapshot` instead of waiting unused, and m285's README needed no pointer. The store's count of accepted captures (m285) stays beside it.
  - t006: the leg is `rate` of `sandbox-measure.yml` (`scripts/measure-sandbox-rate.sh`), which runs through the same live wrapper as the isolation matrix, not a leg of the matrix itself. Production runs two bex-api replicas with a bucket each, so the leg does not expect exactly one refusal at the 101st create: it sends 100 more creates than the replicas' buckets hold together and bounds what is admitted. The exact 101st is asserted by the router test.
  - t003 step 4's router tests run against the composed server with fakes for OpenSandbox and Hydra; they need no database.
- **Render parity (t007).** Render's table was read again on 2026-11-20 and is unchanged: seven rows, per user, shared across the user's API keys, 429. REST, GraphQL and MCP spend the same bucket and refuse with the general limiter's shape on each surface. Drift, each looser than Render or bex-owned: one bucket per bex-api replica ([w22/021](../../021.md)), the 10,000-a-minute workspace bound awaiting sign-off ([w22/022](../../022.md)), and a token bucket's burst (Render documents no window; recorded in ADR104 D7, no follow-up).
- **Simplify (t008).** Applied: the sandbox package's own key-creator interface and five-minute cache were replaced by `core.KeyCreatorResolver`; `Middleware` and the batch's `admit` share `reserve` and `generalBuckets`; `core.ReserveEach` takes the token count. One limiter implementation (`core.KeyedRateLimiter`) and one charging helper for GraphQL and MCP (`RateLimits.charge`); REST charges in the general limiter by route pattern. Nothing else found.
- **Test coverage (t009).** Mutations, each caught:
  - `read` at 401 instead of 400: `TestEveryClassAdmitsRendersRateThenRefusesAndRefills` and three more fail.
  - `POST …/terminate` classified `create`: `TestRequestBucketsClassifiesOnlySandboxRoutes` and `TestSandboxClassesAndTheGeneralBudgetAreSeparate` fail.
  - A classified route also charged the general budget: `TestASpentGeneralBudgetDoesNotRefuseSandboxCalls` and four more fail.
  - A batch member without its own scope: `TestAGraphQLBatchRefundsEachSandboxOnlyMember` fails.
  - A mounted route removed from the class table: registration panics, and `TestEveryLiveSandboxOperationHasARateClass` fails.
  - `make lint` is clean for all four modules.

## Gate cleared 2026-11-23 (/loopx w22): the leg ran, and execs are held by the gateway, not the rate limiter

- **Source:** verified evidence, not a user decision. Production's pin `9fdf7e0a1171` (`cded300699`, 2026-11-23T17:33Z) carries `b7bc85e30` (`git merge-base --is-ancestor b7bc85e30 9fdf7e0a1171` succeeds). Production has carried it since pin `1a5a8e872398` (04:25Z). No `rate` leg had run before: every `sandbox-measure` run on 2026-11-21 and 2026-11-23 was the `usage` leg.
- **Run:** [45279460837](https://github.com/bex-co/bex/actions/runs/45279460837), dispatched with `gh workflow run sandbox-measure.yml -f leg=rate`. It failed 2 of its 10 checks. The figures are under "Production check" below. It did not meet w22/028's reset: every exec answered.
- **Remaining gate:** [w22/m623](../m623/README.md) (promoted from w22/029 on 2026-11-24). Nothing is needed from the user. The ssh-gateway holds one user to 5 concurrent sessions per replica (2 replicas), in a limiter shared with native SSH. A sixth concurrent exec on a replica is refused, and bex-api answers it 400 `too many concurrent exec sessions`. Render documents no such limit (render.com/docs/sandboxes-limits, read 2026-11-23). On production this holds one user near 300 short execs a minute, so the definition of done's "600 execs inside a minute" cannot hold until w22/029 ships.
- **Next:** w22/m623 shipped in `f27106b204` (2026-11-24), which this milestone's gate names as ``pin must carry `f27106b204` ``. Once production runs it, `gh workflow run sandbox-measure.yml -f leg=rate` must print PASS. Then record the run here, flip the ADR018 row to ✅ and close with t010.

## Production check

[Run 45279460837](https://github.com/bex-co/bex/actions/runs/45279460837), 2026-11-23 22:26Z, production pin `9fdf7e0a1171`, one disposable workspace whose owner holds two API keys. 8 of 10 checks passed.

- **Creates: pass.**
  - 400 creates were sent in 12.7 s, alternating between the user's two keys. 120 were admitted by the rate limiter, then refused by the group's concurrency limit as the leg intends; none started a sandbox. 280 were refused with `rate_limited`: 141 on the first key and 139 on the second.
  - The admitted 120 are one bucket of 100 plus 12.7 s of refill at 100 a minute (about 21). One bucket was spent across both keys and both bex-api replicas, which fits w8/m579's shared buckets that this pin also carries.
  - The 429 carried `Retry-After: 1`. Every answer was the rate limiter's 429 or the group's, and the leg left no sandbox behind.
- **Execs: the rate limiter passed; the leg failed.**
  - All 600 mints answered 201 and none was rate limited. All 600 redeems were answered too, none with 429, so the redeem route's former 60 a minute per address is gone.
  - 206 redeems answered 400 from the gateway's session cap (w22/029) and 394 streamed exit code 0. The burst took 76 s.
  - Failed checks: "sent inside a minute" (76 s) and "at least 95% completed" (394 of 600).

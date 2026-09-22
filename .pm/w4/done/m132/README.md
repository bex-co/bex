# w4 · m132 — Make unapplied Postgres overrides visible

**Worker:** worker4 **Goal:** Show declared settings alongside runtime evidence **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add shared declared parameter observations — **DONE** | 30m | — |
| t002 | Show observations beside saved overrides — **DONE** | 30m | t001 |
| t003 | Render parity and documentation — **DONE** | 30m | t002 |
| t004 | Simplify — **DONE** | 30m | t003 |
| t005 | Test coverage — **DONE** | 30m | t004 |
| t006 | Closeout — **DONE** | 30m | t005 |

## Definition of done

REST, GraphQL, MCP and the dashboard expose each saved override alongside non-default runtime observation or an explicit absence/unavailability signal. Unit normalization does not create false mismatch claims. Runtime failure does not hide declared settings. Relevant suites pass.

## Source + Goal linkage

- **Source:** [w4/115 evidence](source-115.md).
- **Goal linkage:** ADR008 dependable managed hosting and API/agent usability.
- **Expected outcome:** Users can distinguish saved configuration from observed configuration.
- **Why now:** Live QA found invalid values silently stored without evidence they applied. End-to-end API/UI scope exceeds one hour, so promoted.
- **Render parity:** Included because REST/GraphQL/MCP/UI change; diagnostics are additive bex behavior, not a new Render parity claim.

## Verification in progress

- Real PostgreSQL tests pass: normalized 8MB / 8192 kB, previous effective values, unknown declared names, runtime connection failure. Kubernetes is fake; no CNPG apply or production probe is claimed.
- Backend unit and three transport checks pass; permission revocation propagates.
- Dashboard editor and polling checks: 15 tests pass; TypeScript and targeted ESLint pass; translation/locale checks: 148 pass.
- Three simplify reviews completed; map lookup and visible-page observation polling applied.
- Backend lint: zero issues. Mutation disabling observations makes the real-Postgres regression fail as expected.
- First full dashboard run overlapped a translation-key rename (one failure); rerun underway on final files. First backend run hit Docker OpenFGA timeout errors; replacing the local dependency with a separately owned native instance before retry.

## Final verification — 2026-09-22

- Full backend `go test -p 1 ./...` passes with real native PostgreSQL, native OpenFGA and local OpenBao. Log: `/tmp/bex-w4-m132-backend-final.log`. An overlapping API retry had collided with store fixture cleanup; the final run is sequential and passes.
- Full dashboard suite: 434 files, 3587 tests pass (`/tmp/bex-w4-m132-dashboard-final.log`).
- Backend lint zero issues; dashboard TypeScript and targeted ESLint pass. All three simplify reviews complete.
- Mutation disabling observation fails real-Postgres regression as expected. No production rollout or CNPG apply probe claimed; this milestone reports runtime evidence rather than validating writes.

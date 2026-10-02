# w8 · m48 — Close the log filtering and Blueprint validation gaps

**Worker:** worker8 **Goal:** tenant logs contain useful PostgreSQL/App output, and Blueprint validation advertises its actual bounded input size. **Status:** done (2026-10-02)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Repair Postgres record and kubelet-placeholder log filtering — **DONE** | 45m | — |
| t002 | Align Blueprint validation with the existing 512 KiB compiler limit — **DONE** | 25m | — |
| t003 | Render parity across exposed surfaces — **DONE** | 15m | t001, t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage and live acceptance — **DONE** | 40m | t003, t004 |
| t006 | Closeout — **DONE** | 10m | t005 |

## Definition of done

- CNPG PostgreSQL records retain their line and severity; non-record PostgreSQL messages never become `<no value>` templates, and CNPG instance-manager/probe noise is excluded.
- Missing-container kubelet placeholder bodies do not flood tenant App logs; valid tenant lines continue through the same rendered pipeline.
- The existing 512 KiB pre-decode Blueprint guard remains. Validation transport errors and documentation agree, including 600 KiB and 3 MiB inputs, boundary-size content, and multipart overhead.
- Relevant tests/lint and local live acceptance pass with concrete evidence and owned-fixture cleanup. Production rollout is not implied by local verification.

## Source + Goal linkage

- **Source:** [w8/032](../032.md), CLI QA sweeps 51/53 and the 2026-09-28 follow-up. Promoted 2026-10-02 after confirming all three residuals in current code.
- **Goal linkage:** ADR008 hosting usability, ADR010 tenant observability, ADR049 bounded Blueprint parsing, and ADR018 Render compatibility.
- **Expected outcome:** log readers see tenant PostgreSQL/App lines without platform noise; Blueprint callers receive the same truthful size limit before parsing.
- **Why now:** the original shipping fixes left confirmed customer-visible residuals, and larger Blueprint requests currently get inconsistent 512 KiB versus 10 MiB messages. Preserve the established security budget rather than expanding it.
- **Scope:** two implementation tasks plus the standing Render parity, simplify, test, and closeout tasks. Render parity is included because tenant log output and validation errors are API-visible.

## Acceptance

[Verified results and limitations](acceptance.md): full backend and lint checks, five pinned Alloy cases, 50 live local checks, and a sanitized cleanup ledger. Production rollout is not claimed.

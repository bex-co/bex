# w2 · m166 — Clearing a datastore allowlist must block external access

**Worker:** worker2 **Goal:** the Render-compatible `--clear-ip-allow-list` operation disables external access to Key Value and Postgres instead of removing the network restriction, while internal access and unrelated network layers keep their contracts. **Status:** done (2026-10-02; local process acceptance, with substrate limits recorded in evidence/acceptance.md)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Repair explicit-empty datastore access intent — **DONE** | 50m | — |
| t002 | Verify every writer, proxy route, and legacy-state boundary — **DONE** | 45m | t001 |
| t003 | Render parity and accurate networking copy — **DONE** | 25m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Behavioral coverage and live CLI acceptance — **DONE** | 50m | t003, t004 |
| t006 | Closeout — **DONE** | 10m | t005 |

## Definition of done

- On owned free Key Value and Postgres fixtures, source rules permit new authenticated external connections; a nonmatching rule denies them; `keyvalues/postgres update --clear-ip-allow-list` exits 0 and, after bounded convergence, blocks new external connections. This must hold for existing credentials and direct datastore clients, not just a CLI-side refusal.
- A later explicit nonempty allowlist restores access only to its allowed sources. Omitted fields leave intent unchanged; preview performs no writes; invalid CIDRs and denied writes preserve the previous state.
- All eight logical API writer surfaces (PATCH, dedicated PUT, GraphQL and MCP for each datastore), plus both Blueprint datastore update paths, share the intended semantics. Blueprint create with explicit rules and export/adoption preserve effective access intent. Document explicit Bex `public` precedence, create defaults, and treatment of pre-existing public/empty resources without a blind global policy change.
- The internal database path stays reachable. Postgres primary, pooler and read-replica route variants enforce the intended external rule. Environment AND composition, inherited deny-all, and service/static HTTP empty-list behavior remain correct.
- CLI text saying external connections are blocked agrees with the actual network result. REST/GraphQL/MCP/dashboard and ADR009/ADR021/checklist wording agree with the chosen datastore contract; the old claim that Render's empty datastore list means open is corrected.
- Appropriate backend/operator tests pass; live acceptance records both admission and denial with valid credentials, verifies owned-fixture cleanup, and distinguishes tested paths from inferred coverage.

## Source + Goal linkage

- **Source:** User-requested infinite `$qa-find-bugs-cli` hunt in `w2`, 2026-10-02. [Complete sanitized finding](finding.md). Severity **major**. No implementation performed.
- **Goal linkage:** Render-compatible hosting and truthful network controls (ADR008, ADR006, ADR018, ADR009, ADR021). The exact pinned CLI and official Render datastore documentation both define empty as externally blocked.
- **Expected outcome:** Automation that disables datastore external access actually closes that network path, and users can trust the CLI's success/readback.
- **Why now:** Live CLI clear reports success and blocked text while the external endpoint accepts authenticated traffic. Existing w7/m45 coverage checked array mutation; w4/m116 intentionally kept publishing sticky based on the opposite Render assumption. This filing supplies the missing network proof and correct contract, rather than reopening their successful write/publish fixes indiscriminately.
- **Scope:** One cross-datastore semantic defect with two traced backend writers and a shared proxy predicate; one repair task plus a counted blast-radius task. About three hours including standing closing tasks. Render parity is required because network intent and user-facing copy cross every surface. Credentials remain required: this finding is not an authentication bypass or evidence of data compromise.

## Production CLI follow-up — 2026-10-03

The ongoing user-requested CLI hunt verified the external allow → clear → restore sequence on one owned free Key Value and one owned free Postgres primary in `bex-canary`. Both returned success with empty allowlists after the installed CLI's `--clear-ip-allow-list`; fresh connections using the saved pre-clear endpoint and credentials then failed. Restoring the exact two source rules restored authenticated `PING` / `SELECT 1`. Both fixtures were deleted, absent from lists, not found by get, and unreachable through their former external endpoints. [Sanitized commands and outcome matrix](evidence/cli-production-access-clear-20261003.json).

The installed v0.2.1 Postgres client still has the separately tracked CA-trust limitation; the checkout launcher `dev-f296747d9` and direct `psql` with the supplied CA established the successful baseline. Early provisioning failures were excluded. This supplements external primary-path evidence only: it does not repeat internal access, replicas, poolers, all writer surfaces, or the complete milestone DoD. No new implementation issue or status change.

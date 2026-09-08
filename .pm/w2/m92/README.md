# w2 · m92 — ADR055 F2/F3: execute the production identity migration (phases 1–3, arm phase 4)

**Worker:** worker2 **Goal:** every labeled App's registry identity (Zot repo/user/pull Secret) and static prefix is workspace-scoped in production, closing the security register's two open HIGH findings to "phase 4 armed" — with the runbook's STOP honored at every mutating phase. **Status:** todo

## STOP — authorization model

Phases 2, 3, and 4 mutate live tenant artifacts. Per `docs/runbooks/registry-static-identity-migration.md`, **no mutating phase runs without explicit operator (user) authorization in a change window** — materializing this milestone schedules the work; it does not grant that authorization. t003 and t004 are blocked until their authorization is given and recorded in the task file. Phase 1 (inventory) is read-only and needs none.

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Phase 1: read-only inventory of legacy-named identities across prod            | 40m | —          |
| t002 | Rehearse phase 2 `--apply` on a scratch App (runbook precondition)             | 40m | t001       |
| t003 | Phase 2 (STOP-gated): `registry-migrate --apply` — copy, verify, tombstone     | 45m | t002       |
| t004 | Phase 3 (STOP-gated): tenant redeploy onto scoped refs; verify every App healthy | 45m | t003       |
| t005 | Arm phase 4: record the 14-day clean window; annotate ADR055; file the follow-up note | 20m | t004 |
| t006 | Simplify                                                                       | 20m | t005       |
| t007 | Test coverage                                                                  | 30m | t005       |
| t008 | Closeout                                                                       | 10m | t007       |

## Definition of done

`scripts/verify-workspace-scoped-identity.sh` (or the equivalent inventory procedure) shows **zero** labeled Apps still on legacy-named registry repos/users/Secrets or legacy static prefixes; every migrated artifact was digest-verified before tombstoning and **no blob was deleted** (phase 4's job). Every App is healthy after the phase-3 redeploy, serving from its `W/A` image ref / `W/A/<rev>/` static prefix. `docs/ADR055-security-review-round4.md` F2/F3 rows are annotated "phases 1–3 executed `<date>`, phase 4 armed `<date>`" and the runbook records the clean-window start. A follow-up inbox note exists for phase 4 (drop dual-read + delete legacy blobs after the 14-day clean window). Each mutating phase's explicit user authorization is recorded in its task file. Rollback paths were never needed, or their use is documented.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-09-07 #2 (approved by user 2026-09-07). `w2/done/m75`'s closeout: "F2/F3 stay open until runbook phase 4"; `docs/ADR055-security-review-round4.md` findings table rows F2/F3 ("**Deferred — migration-gated**"); the STOP-gated runbook `docs/runbooks/registry-static-identity-migration.md`.
- **Goal linkage:** tenant isolation (ADR043 lineage) — F2/F3 are the security register's two HIGH findings: registry repos/users and static-site prefixes keyed by App name alone collide across workspaces, so one tenant's artifacts are reachable under another's same-named App.
- **Expected outcome:** production runs entirely on workspace-scoped artifact identities (ADR074); the only remaining step to fully close F2/F3 is the phase-4 fallback drop, armed with a dated clean-window clock and a filed follow-up.
- **Why now:** the code, dual-read, migration tool, and runbook have been ready since 2026-08-18 (w2/m75); every deploy since grows the legacy artifact set the migration must copy — the work only gets bigger. With 17 first-party workspaces the blast radius is as small as it will ever be.
- **Render parity omitted:** platform storage identity migration — no REST/GraphQL/MCP/UI wire change (artifact names are internal; the dual-read seam guarantees serving behavior is unchanged).

# w4 · m32 — Environment IP-layer lifecycle correctness (cross-feature edges + backfill)

**Worker:** worker4 **Goal:** the environment inbound-IP layer m28 made enforcement-bearing survives ordinary project-membership lifecycle: pulling a service out of its project clears its layer (no frozen deny-all), deleting a project un-stamps every member, and environments that predate m28 actually enforce their existing rules on member Apps. **Status:** DONE 2026-07-16

## Tasks (in order)

| id   | title                                                                                                                | est | depends_on | status |
| ---- | -------------------------------------------------------------------------------------------------------------------- | --- | ---------- | --- |
| t001 | `SetProjectServices` clears `environment_id` for departing services + one-shot sweep for already-drifted rows         | 45m | —          | DONE |
| t002 | `projects.Delete` fans the environment-layer clear to member Apps/DBs/KVs of cascaded environments                    | 45m | t001       | DONE |
| t003 | Idempotent backfill sweep: re-run the fan-out for every environment; drop byte-identical datastore clobber residue    | 60m | t002       | DONE (residue-drop declined, see t003) |
| t004 | Render parity — lifecycle semantics match Render's documented environment-rule behavior across surfaces               | 30m | t003       | DONE |
| t005 | Simplify — `/simplify` over the changed code                                                                          | 20m | t004       | DONE |
| t006 | Test coverage — lifecycle-edge and idempotency tests asserting real enforcement outcomes                              | 45m | t004       | DONE |
| t007 | Closeout — verify DoD, sync status, move to done                                                                      | 15m | t006       | DONE |

## Definition of done

Pulling a service out of its project clears `apps.environment_id` and the stamped `spec.environmentIPAllowList` (no frozen rules or deny-all; table-driven test); deleting a project leaves no member App/DB/KV CR carrying a cascaded environment's `spec.environmentIPAllowList`; after the backfill sweep, an environment with pre-m28 non-empty rules demonstrably enforces on a member App that predates m28 (rules present in the projected CR); both sweeps are idempotent — a second run is a no-op, asserted by test.

## Source + Goal linkage

- **Source:** promotes `.pm/w4/025.md` (filed by `w4/m28`'s consistency review, 2026-07-15) via `/pm-brainstorm for w4 focusing on polishing existing features`, 2026-07-15. Evidence, all verified in code at filing: `projects.SetServices` (`lego/backend/internal/projects/service.go` → `store/projects.go:107`) leaves departing rows' `environment_id` stale while `ListEnvironmentServices` (`store/environments.go:195`) filters on both columns, so the stamped layer freezes; `DELETE FROM projects` cascades child environments without any member-CR clear; pre-m28 members carry no layer until the environment's next incidental write.
- **Goal linkage:** tenant isolation (docs/ADR022-tenant-isolation.md) + protected-environment parity (docs/ADR032-environments.md § Inbound IP rules) — w4's multi-tenant-security charter.
- **Expected outcome:** environment IP rules can no longer end up silently unenforced (pre-m28 members) or frozen/unclearable (project-membership changes, project deletion) through ordinary lifecycle; the dashboard editor controls what it appears to control in every lifecycle state, not just steady state.
- **Why now:** m28 shipped enforcement 2026-07-15, which converted these ADR032-documented label-only staleness edges into live security-semantics gaps — the same class of "UI implies enforcement the backend doesn't deliver" that justified m28 itself. Edge 3 is the sharpest: environments with pre-m28 rules look protected in the dashboard but aren't enforced on their Apps until an unrelated write happens to touch them.
- **Render parity:** included, narrowly — no new wire shapes; the check is that lifecycle semantics (service leaves environment ⇒ rules stop applying; project deleted ⇒ members unblocked) match Render's documented behavior (`docs/render-artifacts/protected-environments.md`), and that REST/GraphQL/MCP + dashboard observe identical post-lifecycle state.
- **Coordinate with:** `w4/m28`'s t008 closeout (same feature thread — its live prod verification should land first or alongside) and `scripts/ipallowlist-normalize.sh` (t003 reuses its one-shot-sweep shape).

# w21 · m394 — Security review round 25: the operator's tenant-facing daemons and the dashboard's server routes

**Worker:** worker21 **Goal:** every operator daemon that parses tenant-controlled input and every dashboard server route has been reviewed with the `security-audit` skill, each confirmed finding is fixed with a regression test that fails on the old code, and ADR109 records scope, findings and accepted residuals. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Scope the operator plane and run the audit on its daemons | 90m | — |
| [t002](t002.md) | Run the audit on the dashboard's server routes and its OAuth consent and device flows | 60m | — |
| [t003](t003.md) | Reproduce each finding and reject what does not hold | 60m | t001, t002 |
| [t004](t004.md) | Fix each confirmed finding with a regression test that fails on the old code | 120m | t003 |
| [t005](t005.md) | Write ADR109 and update the review lineage | 45m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 20m | t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

1. `docs/ADR<next>-security-review-round25.md` exists with a scope table that gives a verdict for each `lego/operator/cmd/*` binary and each dashboard server route named in t001 and t002, and `bash scripts/adr-registry-validate.sh` passes.
2. Every confirmed finding has a regression test that fails on the pre-fix code and passes after the fix.
3. `make test` and `make lint` (from `lego/operator/`), `cd lego/backend && go test ./...` and `cd dashboard && yarn lint && yarn test` pass.
4. The accepted residuals (`#PSL`, `#CI-RUNNERS`, `#RUNNER-HOSTS`, `#ENV-REVIEWERS`, `#APPROVAL-MODE`) are reported as accepted, not refiled; anything provable only on a live deployment is listed with the check that settles it.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w21` (2026-11-14), filed by `/pmx them all to w21` on 2026-11-14, re-checked at filing against `origin/main` `cc671cbfd`, item 1. ADR107 § Residuals (Coverage) names the operator and the dashboard's server-side routes as the areas round 24 covered least; no board item picks that up. A search of the 24 review ADRs finds `kv-internal-proxy`, `egress-meter`, `auth.consent`, `auth.device` and `api.sessions` named in none.
- **Goal linkage:** V0 roadmap goal 7 (security review) and the review lineage ADR028 → ADR107 in `AGENTS.md`.
- **Expected outcome:** Every surface above has a recorded verdict, and each exploitable issue found is fixed under a test that would catch its return.
- **Why now:** ADR107 says a single run finds roughly half of what repeated runs find and asks the next one to weight these areas; round 24 closed on 2026-11-13, so the findings and the method are fresh. The operator daemons are the public-facing parsers of tenant-controlled input (SNI, HTTP, static files) and `kv-internal-proxy` is newer than every review that touched Key Value.
- **Render parity task:** omitted. This is security hardening of operator and dashboard-server internals. If a fix in t004 changes a tenant-facing response, header or status, `/pm add-task` adds the check for that case.

## Open decisions

None. A finding that would need a new product surface (for example an approval flow) is filed as a finding with the rule in `.pm/DO_NOT_DO.md` it runs into, not built here.

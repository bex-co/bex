# w2 · m89 — Secretless image build, credentialed deploy (ADR080 F2)

**Worker:** worker2 **Goal:** the CI job that compiles code and builds images runs with zero deploy credentials in scope; a separate environment-gated deploy job consumes the exact built digest — closing ADR080 finding 2, still recorded open by ADR083 **Status:** in progress 2026-09-07 — t001–t003 done (split + digest-output handoff + validator check 8 + ADR closures, both validators green locally); t004 awaits the first live run of the split pipeline (triggered by this milestone’s own ship)

## Tasks (in order)

| id   | title                                                                       | est | depends_on |
| ---- | ---------------------------------------------------------------------------- | --- | ---------- |
| t001 | Split `deploy.yml` into a secretless build job and an environment-gated deploy job — **DONE** | 60m | —          |
| t002 | Image handoff by pinned digest between the jobs — **DONE**                   | 45m | t001       |
| t003 | Validator + docs: build jobs must reference no deploy secrets — **DONE**     | 30m | t002       |
| t004 | Verify a full production deploy through the split pipeline                   | 30m | t003       |
| t005 | Simplify                                                                     | 20m | t004       |
| t006 | Test coverage                                                                | 30m | t004       |
| t007 | Closeout                                                                     | 15m | t005, t006 |

## Definition of done

- In `deploy.yml`, no step that checks out/compiles/builds images can read deploy credentials (kubeconfigs, infra tokens, gitops keys): they are neither in that job's `env`/`secrets` scope nor its `environment` gate.
- The deploy job consumes the build's exact image digest (no mutable-tag re-resolution between build and deploy).
- `scripts/github-actions-validate.sh` fails closed if a build-classified job references a deploy-classified secret.
- One real production deploy has shipped end to end through the split pipeline.
- ADR080 finding 2 and ADR083 follow-up 3 are recorded closed with this milestone as owner.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-09-01 #4; `docs/ADR080-security-review-round19.md` finding 2, recorded "still open" by `docs/ADR083-security-review-round20.md` §Follow-ups item 3 — no board item owned it.
- **Goal linkage:** V0 roadmap item 7 (security review); bounds ADR083 finding 4 (runner compromise = platform takeover) at the pipeline's most attacker-influenced stage (the build, which executes repo-controlled code).
- **Expected outcome:** compromise of the build stage loses its highest-value prize (production credentials); composes with w2/m88 — the split jobs map naturally onto the split `ci`/`production` runner pools.
- **Why now:** same custody-shift window as m88; restructuring `deploy.yml` once for both changes avoids doing it twice. Sequence after or alongside m88 (m88's classification table feeds t003's secret classes).
- **Render parity:** **omitted** — pure CI; no REST/GraphQL/MCP/dashboard surface.

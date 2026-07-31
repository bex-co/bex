# w7 · m65 — CI supply-chain hardening: SHA-pin actions + pinning guard + dashboard dependency scanning

**Worker:** worker7 **Goal:** a retagged upstream GitHub Action cannot alter bex's CI, and a known-vulnerable dashboard JS dependency turns CI red — closing the two gaps the 2026-07-31 supply-chain sweep found in an otherwise strong posture. **Status:** done — **DONE 2026-07-31**

## Tasks (in order)

| id   | title                                                                                | est | depends_on       | status |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------------- | ------ |
| t001 | SHA-pin all 42 third-party `uses:` refs (+ sync the reviewed inventory)               | 60m | —                | — **DONE** |
| t002 | Pinning guard: 40-hex enforcement in `github-actions-validate.sh` + red/green self-test | 30m | t001             | — **DONE** |
| t003 | `dependabot.yml`: github-actions + npm (dashboard) version updates                    | 30m | t001             | — **DONE** |
| t004 | Dashboard dependency audit gate in `dashboard-test.yml` (CRITICAL gates, HIGH warns)  | 45m | —                | — **DONE** |
| t005 | Simplify pass over the changed workflows/scripts                                      | 20m | t002, t003, t004 | — **DONE** |
| t006 | Test coverage: guard self-tests assert real red/green behavior                        | 30m | t002, t003, t004 | — **DONE** |
| t007 | Closeout                                                                              | 10m | t006             | — **DONE** |

## Outcomes

- **t001** — All 42 third-party `uses:` entries across the 14 workflows are pinned to full 40-hex commit SHAs with `# vX.Y.Z` comments (16× checkout, 6× trivy-action, 5× setup-go, 3× build-push-action, 2× cache, 2× setup-node, 8 singles); the 3 local `./` reusable-workflow refs are untouched. Each SHA was peeled from its tag via the GitHub git-refs API (annotated tags dereferenced to the commit) and every one re-verified as a real, fetchable commit. `scripts/github-actions-validate.sh`'s `expected_actions` inventory rewritten to the SHA refs.
- **t002** — The validator now fails any third-party ref that isn't a full 40-hex SHA (fully-commented lines and `./` local refs exempt), keyed off an override-honest `WORKFLOWS_DIR` seam. New `scripts/github-actions-validate.test.sh` proves red (tag / 39-hex / 41-hex / Node 20) and green (pin / local ref / commented ref / real tree), and is wired into `scripts.yml`.
- **t003** — `.github/dependabot.yml` adds weekly grouped `github-actions` (root) + `npm` (`/dashboard`) update PRs; header records the `w1/006` triage boundary and the intended fail-closed interplay (an actions bump turns the validator red until `expected_actions` is human-reviewed and bumped).
- **t004** — `dashboard-test.yml` runs `yarn npm audit` twice after install: HIGH reports via `continue-on-error` (currently a transitive `brace-expansion` advisory, recorded for `w1/006`), CRITICAL gates the job. Exit semantics verified against the real `yarn.lock` (HIGH→1, CRITICAL→0). Because `deploy.yml` calls this workflow via `workflow_call`, the gate transitively blocks image build/push — its `needs:`/`uses:` wiring confirmed intact, no deploy change needed.
- **t005** — `/simplify` ran 4 parallel review agents (reuse/simplification/efficiency/altitude). Reuse + efficiency clean; applied the redundant grep-prefilter collapse in `third_party_refs()` (6→5 stages) and the honest-seam refactor (fixture mode keys on "was an override supplied?", not a path string-compare).
- **t006** — Durable dashboard-gate structural check (deletion / severity-downgrade / inline `|| true` neuter all turn the self-test red) added; every shipped guard spot-checked red-on-regression via a throwaway workflow file (SHA-pin, inventory-diff both directions, Node 20). No tautologies.
- **t007** — DoD verified against the real tree; all 14 pinned SHAs confirmed real commits, validator + self-test green, YAML parses. Final CI-green-on-`main` lands with the next `/ship` (push-gated).

## Definition of done

- Every third-party `uses:` entry in `.github/workflows/*.yml` (42 today: 16× `actions/checkout`, 6× `aquasecurity/trivy-action`, 5× `actions/setup-go`, 3× `docker/build-push-action`, 2× `actions/cache`, 2× `actions/setup-node`, 8 singles) is pinned to a full 40-hex commit SHA with a trailing `# vX.Y.Z` comment; the 3 local reusable-workflow refs (`./.github/workflows/*.yml`) are unchanged.
- `scripts/github-actions-validate.sh` fails on any non-SHA third-party ref and still enforces the reviewed inventory diff; a committed self-test proves the guard goes red on a tag-pinned ref and green on the real tree.
- `.github/dependabot.yml` exists with `github-actions` (keeps SHA pins fresh via PRs) and `npm`/`dashboard/` ecosystems; alert/PR triage stays `w1/006` per the w7 README boundary.
- `dashboard-test.yml` runs a dependency vulnerability audit: CRITICAL advisories fail the job (and therefore gate `deploy.yml`, which calls this workflow), HIGH advisories are reported without failing — the m10 Trivy policy.
- All 14 workflows still pass on main after the pinning (checkout/setup actions resolve at their pinned SHAs).

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w7` round 3, 2026-07-31 — CI supply-chain sweep: 0/42 third-party `uses:` entries SHA-pinned (all float on mutable tags, the 2025 `tj-actions/changed-files` attack vector) while `deploy.yml` holds `contents: write` + registry + image-signing credentials; and zero JS dependency scanning (Go has govulncheck via w7/m62, images have Trivy via w7/m10, the dashboard's npm tree has nothing — no `dependabot.yml` either).
- **Goal linkage:** w7's CI-guard charter (the m7/m10/m30/m58/m60/m62 lineage); supply-chain integrity of the pipeline that signs and ships every production image. m62 checksum-pinned the CI *binaries* (govulncheck, gitleaks) — this closes the same class one layer up, and makes CI consistent with bex's own fail-closed digest-pinning rule for production images.
- **Expected outcome:** a compromised/retagged upstream action tag cannot execute in bex's CI; a critical-severity dashboard dependency advisory turns CI red before deploy; both properties are guarded permanently (fail-closed script + self-test), not just established once.
- **Why now:** `deploy.yml`'s write credentials make floating actions the highest-privilege unpinned code on the platform; the fix is cheap and mechanical, and the existing `github-actions-validate.sh` inventory gives the guard a natural home.
- **Render parity:** omitted — pure CI/infra; no REST/GraphQL/MCP/UI surface change.

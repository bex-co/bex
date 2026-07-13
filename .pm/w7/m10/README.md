# w7 · m10 — Security hygiene: image CVE scanning in CI + HTTP hardening headers

**Worker:** worker7 **Goal:** Every build pushes through a CVE gate (CI fails on CRITICAL, warns on HIGH) and both browser-facing surfaces (bex-api, dashboard SSR) carry the standard hardening headers — closing the two remaining open `w7` inbox notes. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                        | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Trivy scan step in `.github/workflows/deploy.yml` after image build+push (bex + dashboard images); warn on HIGH, fail on CRITICAL, tune threshold after a first baseline run (`w7/001`) | 30m | —          |
| t002 | HTTP hardening-header middleware on bex-api's raw `http.Server` (`X-Content-Type-Options`, `X-Frame-Options`, HSTS behind TLS/prod, a conservative CSP) (`w7/002`)                      | 25m | —          |
| t003 | Equivalent hardening headers on dashboard SSR responses, tuned so the CSP doesn't break asset loading (`w7/002`)                                                                        | 25m | t002       |
| t004 | Acceptance: a CI run produces a scan report and the threshold behaves as configured; curl both live surfaces and confirm the header set                                                | 20m | t001, t002, t003 |
| t005 | Simplify — `/simplify` over the code/config this milestone changed                                                                                                                      | 15m | t004       |
| t006 | Test coverage — meaningful checks for the header middleware (both surfaces) and, where feasible, a CI-checkable assertion for the scan-gate config                                     | 25m | t004       |
| t007 | Closeout — verify DoD met, then move the milestone to `done/`                                                                                                                            | 10m | t005, t006 |

## Definition of done

CI fails on a CRITICAL CVE in either built image (verified against a planted/known-vulnerable baseline or a dry run) and warns without blocking on HIGH; both bex-api and the dashboard SSR responses carry the hardening header set in a live check, with the dashboard's own assets still loading correctly under the new CSP.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones to work on` 2026-07-13 — groups `w7/001` (image CVE scanning, filed `/pm-brainstorm for w7` 2026-07-09) and `w7/002` (HTTP security headers, filed 2026-07-12 security sweep finding #6), both individually sub-hour with no milestone home.
- **Goal linkage:** `GOAL.md` #7 (security review) — direct continuation of `w7`'s hardening track; both notes were filed under `w7` for exactly this purpose.
- **Expected outcome:** two small, independent hardening gaps close in one pass instead of sitting unscheduled indefinitely; clears two of `w7`'s three remaining inbox notes.
- **Why now:** both fixes are small, low-risk, and have sat unscheduled since filing; grouping avoids two separate milestone-creation overheads for genuinely small, independent fixes (the same pattern `w1/m23` used for four sub-hour notes).
- **Render parity closing task: omitted** — pure CI/infra + response-header hardening, no REST/GraphQL/MCP/UI surface change.

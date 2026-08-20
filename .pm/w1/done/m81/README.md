# w1 · m81 — Dashboard QA-walkthrough bug sweep: surface mutation errors + close validation/parity gaps

**Worker:** worker1 **Goal:** every dashboard mutation that the API rejects tells the user why (no silent failures), and the free-tier create/scaling surfaces stop letting a customer build an invalid request — closing the four bugs found in the 2026-08-19 customer-perspective QA walk of dashboard.bex.co. **Status:** done

## Tasks (in order)

| id   | title                                                                                      | est | depends_on               |
| ---- | ------------------------------------------------------------------------------------------ | --- | ------------------------ |
| t001 | Surface server-side GraphQL errors in the Add Custom Domain dialog — **DONE**              | 45m | —                        |
| t002 | Diagnose + fix the React #418 hydration mismatch on every dashboard page — **DONE**        | 60m | —                        |
| t003 | Env change → Deploys list: show the rollout or record the parity divergence — **DONE**     | 45m | —                        |
| t004 | Free-tier create/scaling forms: validate plan limits before submit — **DONE**              | 45m | —                        |
| t005 | Render parity across REST/GraphQL/MCP/UI — **DONE**                                         | 30m | [t001, t002, t003, t004] |
| t006 | Simplify the touched dashboard code — **DONE**                                              | 30m | t005                     |
| t007 | Test coverage for the fixed behaviors — **DONE**                                            | 45m | t005                     |
| t008 | Closeout — **DONE**                                                                         | 10m | t007                     |

## Definition of done

- Submitting a domain the API rejects (wildcard, apex/PSL-rejected, already-claimed) in the Add Custom Domain dialog shows the server's actual reason inline and persistently; the dialog no longer relies only on a transient toast. **Met.**
- No `Minified React error #418` (hydration text-content mismatch) on a normal authenticated page load; root cause named. **Met** (root cause identified + fixed + regression test; live prod re-check is post-deploy).
- An env-triggered rollout is discoverable, or the intentional Events-only divergence is recorded in ADR018. **Met** (documented divergence).
- The free-tier Postgres create form rejects a truly out-of-range disk size before submit. **Met** (scaling needed no change — see Outcome).
- Regression tests assert the behaviors and fail on the pre-fix code. **Met.**

## Outcome (2026-08-19)

Implemented in one session; **dashboard test suite green (2317 tests, CI-equivalent), typecheck + lint clean; uncommitted pending `/ship`.** Two of the four QA findings were **corrected during investigation** — the honest results matter more than the original framing:

- **t001 (custom domain) — real, but not "silent."** The error _was_ surfaced, as a **generic transient toast** ("Couldn't add {name}") that vanishes in ~4s and left the dialog looking untouched — my original QA screenshot simply missed it (taken tool-calls later, after the toast auto-dismissed). Re-probed live and confirmed the toast. The genuine gaps: it never told the user _why_ (a wildcard/apex/PSL rejection all collapsed to the generic message), and there was no persistent inline error like the webhook form has. Fix: `classifyAddError` now surfaces the server's actual reason (stripping the `bad request:` prefix) and the dialog shows it inline+persistently, matching the webhook pattern. Files: `use-custom-domains.ts`, `custom-domains-section.tsx`.
- **t002 (#418) — root cause found decisively.** `global-search.tsx` rendered the search shortcut via an **ungated** `navigator.platform` read: SSR (Node on the Linux prod host, `platform` ≠ "MacIntel") rendered **"Ctrl K"**, a Mac browser's first render rendered **"⌘ K"** → a text-node hydration mismatch on **every page** (the search box is in the persistent header). Proven by diffing prod SSR HTML ("Ctrl K") against the live DOM ("⌘ K"). It was **unreproducible locally** because Node v24 on the dev Mac defines `navigator.platform = "MacIntel"`, matching the browser — so local SSR and client agreed. Fix: a mount-gated `isMac` state so SSR and the first client render always agree on "Ctrl K", then swap after mount. Regression test asserts the SSR render is "Ctrl K" even with `navigator.platform` forced to Mac (**proven red on the pre-fix code**).
- **t003 (env change not in Deploys) — intentional divergence, documented.** A subagent investigation confirmed bex deliberately reserves the Deploys list for image/build rollouts; an env/config edit (incl. Environment-page "Save and deploy") bumps `spec.restartedAt` to roll once and records an **Event** (`service_environment_changed`), opening **no** `dep-…` row — stated intent in `internal/secrets/batch.go`, ADR004, and the deliberately-unset `env_updated`/`zero_downtime_redeploy_*` triggers. Recorded as a divergence in `docs/ADR018-render-parity.md` (Environment & config section). No backend change.
- **t004 (free-tier validation) — premise corrected.** The QA finding assumed per-plan caps bex does not enforce. **Disk:** `storageGB` is a plan _floor_ that grows to a global `diskAutoscalingCapGB: 16384`, so a 9999 GB disk on Free is **below the cap and genuinely valid** — not a bug. The real gap was only the missing client bound; the create form now validates disk against `[plan floor, 16384]` (9999 stays valid, 99999 is blocked). **Scaling:** bex enforces only a global `MaxReplicas` (100), no per-plan replica cap, so a Free service scaling to 100 is backend-accepted — the slider is already correct. Documented as a deliberate divergence from Render (Free is single-instance there) in ADR018's Manual scale row. Files: `create-database-dialog.tsx`, new `databases/lib/disk.ts`, locales.

**Post-deploy follow-up:** re-verify on prod after `/ship` that (a) the #418 is gone across overview/service/billing/metrics and (b) the custom-domain dialog shows the specific reason inline — both are covered by regression tests and decisive root-cause evidence, so this is confirmation, not open risk.

## Source + Goal linkage

- **Source:** 2026-08-19 customer-perspective QA walkthrough of dashboard.bex.co (this session, no prior inbox note). Four findings, re-triaged above.
- **Goal linkage:** Render parity + product quality on the human-facing dashboard (`dashboard/CLAUDE.md`, `docs/ADR006-bex-api.md`, `docs/ADR018-render-parity.md`).
- **Expected outcome:** a customer configuring a custom domain, editing env vars, or provisioning a free-tier resource always gets truthful, immediate feedback; the dashboard stops logging a hydration error on every navigation.
- **Why now:** found in a real customer walk of production; the #418 (every page) and the custom-domain feedback both had clean, isolated root causes. Render parity closing task **included** because every finding touches a user-facing surface.

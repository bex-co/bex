# w8 · m49 — Reject invalid image deploy inputs with actionable errors

**Worker:** worker8 **Goal:** unsupported source selectors and malformed/disallowed images fail before a deploy is created; legacy malformed images receive an operator diagnosis. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Validate OCI image grammar and report registry refusal reasons — **DONE** | 45m | — |
| t002 | Refuse caller commit overrides on image-backed deploys — **DONE** | 20m | — |
| t003 | Diagnose InvalidImageName in rollout and pre-deploy failures — **DONE** | 25m | — |
| t004 | Render parity across exposed surfaces — **DONE** | 15m | t001, t002, t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage and local live acceptance — **DONE** | 40m | t004, t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- A caller commitId on an image-backed service is refused consistently before any deploy row or spec mutation. Internal restarts and repo-backed source selection retain their behavior.
- One image validator enforces repository/digest grammar and the existing registry policy, explaining malformed input, non-public registry addresses, and untrusted registries distinctly. Create, update, Blueprint, and deploy callers use it.
- Valid short/tag/digest references remain usable, including a successful owned local digest deployment.
- InvalidImageName is diagnosed through the existing guarded operator failure paths, without misclassifying transient failures or stale/foreign pods.
- Relevant full suites, lint, adapter regressions, and local live acceptance pass; all owned tenant fixtures are removed. Primary Render evidence and unverified external behavior are recorded honestly.

## Source + Goal linkage

- **Source:** [w8/033](../033.md), CLI QA sweep 54; promoted 2026-10-02 after all three premises remained in current code.
- **Goal linkage:** ADR008 reliable Render-alternative hosting, ADR004 deploy lifecycle, and ADR018 consistent API/CLI behavior.
- **Expected outcome:** callers get actionable input errors without accidental releases, and malformed legacy image refs no longer wait without a diagnosis.
- **Why now:** invalid input currently starts real deploys or fails with a misleading message. Backend input checks and operator runtime diagnosis are separate gaps; the shared m44 lifecycle work does not yet classify InvalidImageName.
- **Scope:** three implementation tasks across backend and operator, plus Render parity, simplify, testing, and closeout. This exceeds a sub-hour note. Registry trust policy, per-deploy override behavior, and the already parked m44 decision record remain as established.

## Completion — 2026-10-02

All seven tasks complete. Full backend/operator suites and all-module lint/deadcode passed; 57 final live checks and eight cleanup checks passed. See [acceptance evidence and limits](acceptance.md), including the unchanged shared operator, retained earlier attempts, and unverified exact Render response.

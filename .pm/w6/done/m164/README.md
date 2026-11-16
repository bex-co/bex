# w6 · m164 — Native-common qualification evidence restored from the pre-purge bundle into object storage

**Worker:** worker6 **Goal:** native-common qualification reads its candidate evidence again, from object storage verified by sha256, with no evidence in git. **Status:** superseded 2026-11-16 by w6/m422 (t001–t003 done; t004–t007 superseded, never run)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Extract every evidence file the test candidate references from the backup bundle and verify it](done/t001.md) — **DONE** | 40m | — |
| t002 | [Upload to object storage and commit a path → sha256/URL manifest](done/t002.md) — **DONE** | 50m | t001 |
| t003 | [evidence_file() fetches a missing path from the manifest and verifies its sha256](done/t003.md) — **DONE** | 60m | t002 |
| t004 | [Dispatch native-common and confirm plan/qualify read the evidence](done/t004.md) — **SUPERSEDED** | 30m | t003 |
| t005 | [Simplify](done/t005.md) — **SUPERSEDED** | 15m | t004 |
| t006 | [Test coverage](done/t006.md) — **SUPERSEDED** | 20m | t005 |
| t007 | [Closeout](done/t007.md) — **SUPERSEDED** | 10m | t006 |

## Definition of done

- native-common's plan and qualify jobs read every candidate evidence file (dispatch run cited) with none in git; `repo-artifact-guard` stays green.
- A tampered evidence object is refused by its sha256.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w6` 2026-11-07, proposal 1; check against `b09eaf24b`: 10 of 10 candidate evidence paths missing after `dcdc774c1`.
- **Goal linkage:** ADR008 reliable delivery and w8/m91's native image custody chain.
- **Expected outcome:** native image publication can qualify again instead of failing at the first evidence read.
- **Why now:** it is already broken; the next native publish hits it. Touches w8's qualification code: coordinate with w8.
- **Render parity:** omitted — CI/infrastructure only, no REST/GraphQL/MCP/UI surface.

## BLOCKED — 2026-11-07: the evidence package must be public

- **Done:** t001–t003 (`b62e7a8d3`); details in `done/`.
- **Gate (user, GitHub UI):** `ghcr.io/bex-co/bex-evidence` was created `internal`; the plan/qualify jobs run on credential-free `bex-ci` runners and pull anonymously, as deploy does for `bex-charts` (public). GitHub has no API to change package visibility. **Action:** bex-co → Packages → `bex-evidence` → Package settings → Danger Zone → Change visibility → Public. The evidence was public in this public repository until 2026-11-04, so this restores, not widens, its exposure. **Done signal:** `curl -s "https://ghcr.io/token?scope=repository:bex-co/bex-evidence:pull"` returns a token.
- **Then (worker):** dispatch `native-common-image.yml` with `source=reviewed-candidate` and confirm plan materializes and reads the evidence (t004).
- **Expected, not this milestone:** `validate_test_candidate` reports "stale qualification source set/bytes/modes" — the qualification sources (`lego/operator/internal/build/**`, the native workflows and scripts) changed since the run-100 baseline. Recording a fresh utility baseline and rerunning the family cases is w8/m91's remaining work, now unblocked by this evidence store.

## Re-triage — 2026-11-12 (w6/119): the gate stands; one alternative recorded

- **Hosted runners make an authenticated pull possible.** `ghcr.io/bex-co/bex-evidence` is `internal` and linked to `bex-co/bex` (`gh api orgs/bex-co/packages/container/bex-evidence`), so a job here could pull it with `GITHUB_TOKEN` and `permissions: packages: read`.
- **That edit is not free.** The pull lives in `.github/workflows/native-common-image.yml` (the `plan`, `qualify` and `qualify-profile` jobs run `contents: read` and pull through oras without logging in). That file is one of the qualification `SOURCE_FILES` in `.github/scripts/native-common-qualify.py`, so editing it reopens the w8/m91 D3 baseline, the same custody gate as m163 and m291.
- **Unblock paths:**
  1. The user makes the package public in Package settings. This needs no code; GitHub has no API for it.
  2. w8 accepts a requalification for an authenticated pull.
- Path 1 stays the recommendation, and t004 is unchanged.

## Superseded — 2026-11-16 (w6/m422)

- **Source:** the user's request of 2026-11-16, "/pmx the deletion, cleanup of references of bex-evidence to w6", filed as [w6/m422](../m422/README.md).
- **What changed:** m422 retired the reviewed-candidate dispatch mode that this evidence store fed (`e9f27399b`). `.github/native-evidence.json`, `.github/native-common-test-candidate.json`, `materialize-evidence` and the candidate validators are gone, and native-common qualifies only the image each run publishes. Push run [43841677055](https://github.com/bex-co/bex/actions/runs/43841677055) on `e9f27399b` passed plan → publish → qualify with no evidence step.
- **The package is deleted:** `ghcr.io/bex-co/bex-evidence@sha256:1203a4e79ae834fff82aee6fcdb658b432cc8335af87a4bb4eba97fb26de9f75` (tag `native-common-m91-20261104`, 1,412 recovered files from `common-current0941-20261101` and `common-d3-run100-20261104`) was deleted on 2026-11-16 (m422 t006). `gh api /orgs/bex-co/packages/container/bex-evidence` answers 404.
- **Not claimed:** t004's acceptance, a dispatch that reads the evidence, never ran. The user gate (make the package public) is moot. t001–t003 stay done as history; t004–t007 are superseded, not done.

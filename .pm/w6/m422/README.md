# w6 · m422 — bex-evidence and the reviewed-candidate path it fed are gone

**Worker:** worker6 **Goal:** native-common qualifies only fresh images, nothing in the repository reads or pins the old 0941 evidence, and the `bex-evidence` GHCR package is deleted. **Status:** in progress (t001–t003, t007, t008 done; t004 waits for the push run)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Retire the reviewed-candidate path in the qualifier and the workflow](done/t001.md) — **DONE** | 90m | — |
| t002 | [Delete the evidence and custody tests; the fresh-path suites stay green](done/t002.md) — **DONE** | 50m | t001 |
| t003 | [Remove the custody pins and exemptions from the repository checks and docs](done/t003.md) — **DONE** | 60m | t001 |
| t004 | [The push-triggered fresh native-common run on the shipped SHA passes](t004.md) | 30m | t002, t003 |
| t005 | [Board: close m164 as superseded, unblock m163, m291 and w8/m91](t005.md) | 40m | t004 |
| t006 | [Delete the `bex-evidence` GHCR package](t006.md) | 10m | t004 |
| t007 | [Simplify](done/t007.md) — **DONE** | 15m | t005, t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 20m | t007 |
| t009 | [Closeout](t009.md) | 10m | t008 |

## Definition of done

- `.github/native-evidence.json`, `.github/native-common-test-candidate.json` and `.github/scripts/test_native_oci_evidence_location.py` no longer exist. `native-common-image.yml` has no `source` input and no evidence step. `native-common-qualify.py` has no `checkout-evidence` / `materialize-evidence` subcommand and no candidate validators.
- `rg -n 'bex-evidence|native-evidence|reviewed-candidate|native-common-test-candidate|D3_LEGACY_0941' .github scripts deploy lego` returns nothing.
- `scripts/github-actions-validate.sh` passes with no native-workflow exemption justified by D3 custody. `scripts/ship-gates.sh` and its self-test pass.
- The push-triggered `native-common-image.yml` run on the shipped SHA succeeds (plan → publish → qualify), and the run id is cited in t004.
- `gh api /orgs/bex-co/packages/container/bex-evidence` answers 404.
- m164 is in `w6/done/` with a superseded verdict. m163 and m291 record that their D3 custody gate is cleared. w8/m91 records the retirement and its next step on fresh images.

## Source + Goal linkage

- **Source:** the user's `/pmx the deletion, cleanup of references of bex-evidence to w6` on 2026-11-16. The conversation before it established the following:
  - `bex-evidence` (`ghcr.io/bex-co/bex-evidence@sha256:1203a4e7…`, tag `native-common-m91-20261104`, `internal`) holds two old CI run packets for native-common image 0941.
  - Its only reader is `native-common-qualify.py materialize_evidence()`, and only in the manual `reviewed-candidate` dispatch mode.
  - Production, the operator, bex-api and the default `fresh` push runs never read it (`nativeCommonBase` in `lego/operator/internal/build/nativebases.go` is empty).
  - Deleting it and its references means retiring the `reviewed-candidate` mode it exists for, so w8/m91 qualifies a fresh image instead of reusing 0941.
- **Goal linkage:**
  - ADR008 reliable delivery.
  - `DO_NOT_DO.md` #NO-ARTIFACTS-IN-GIT: review evidence is not an input CI must replay.
  - It clears the D3 custody gate that holds w6/m163, w6/m291 and w8/m91.
- **Expected outcome:**
  - Editing a native workflow or qualification script no longer reopens a byte-pinned baseline.
  - m164 stops waiting on a user click.
  - m163 and m291 become workable.
- **Why now:**
  - m164 has waited on a user GitHub-UI action since 2026-11-07.
  - The same byte-pin parks m163 and m291.
  - Every native edit since run 100 has made the 0941 baseline stale again (m91 README, 2026-11-04 record), so the evidence could not admit a family case even if it were readable.
- **Decision state:** deleting `bex-evidence` and its references is the user's request of 2026-11-16. How w8/m91 later shows that one digest passed every family case is w8's design and stays open. Running all cases in one fresh dispatch is one option. Passing a digest and checking the publisher's existing keyless signature with `cosign verify` is the other, and it is this proposal's recommendation if reuse is needed. Neither option replays raw run output.
- **Render parity:** omitted. This is CI and infrastructure only, with no REST/GraphQL/MCP/UI surface.

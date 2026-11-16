# w6 · m163 — Native image workflows install Syft and Trivy with retries

**Worker:** worker6 **Goal:** no workflow depends on a single-attempt Syft or Trivy download. **Status:** todo (t001–t006 done; t007 closes on the first native-common push run showing both retrying installers)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [`install-syft.py` takes a version; reviewed sha256s for 1.51.1 and 1.55.0](done/t001.md) — **DONE** | 40m | — |
| t002 | [pg-operand, native-base and native-common use `install-syft.py 1.55.0`](done/t002.md) — **DONE** | 50m | t001 |
| t003 | [native-base installs Trivy once and scans with `skip-setup-trivy`](done/t003.md) — **DONE** | 30m | — |
| t004 | [Drop download-syft from the reviewed action inventory](done/t004.md) — **DONE** | 15m | t002 |
| t005 | [Simplify](done/t005.md) — **DONE** | 15m | t003, t004 |
| t006 | [Test coverage](done/t006.md) — **DONE** | 20m | t005 |
| t007 | [Closeout](t007.md) | 10m | t006 |

## Definition of done

- `git grep download-syft .github` is empty; the validator, its self-test and the native suites (`test_native_oci_*.py`) pass.
- The next run of each changed workflow shows the retrying installers in its log.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w6` 2026-11-06, proposal 5; the 2026-11-05 Syft diagnosis (Option B) and native-common run 42037362778.
- **Goal linkage:** ADR008 reliable delivery: image publication should not fail on one dropped download.
- **Expected outcome:** native and operand image builds survive transient GitHub download failures.
- **Why now:** the host link is degraded now, and these single-attempt installers are the next reds. Touches w8-pinned files: coordinate in the commit and notify w8.
- **Render parity:** omitted — CI/infrastructure only, no REST/GraphQL/MCP/UI surface.

## BLOCKED — 2026-11-06: native-base-image.yml is byte-pinned by w8/m91's D3 custody transition

- **Done:** t001; t002 for pg-operand-image and native-common-image (`d1a92f759`).
- **Gate (w8 / user):** `native-base-image.yml` is in `IMAGE_POLICY_FILES` and pinned byte-for-byte as `D3_LEGACY_0941["after"]` in `.github/scripts/native-common-qualify.py` (`test_d3_transition_pin_binds_the_actual_checked_out_publisher_policy`). Any edit — the Syft installer (rest of t002), the Trivy reuse (t003) — breaks that pin; re-pinning re-approves the legacy 0941 transition, which is w8's custody decision. The same pin already made w9/m173's timeout on its matrix job turn native-common red; `31c371e57` restored the pinned bytes and exempted the file from validator check 10.
- **Release condition:** w8 decides to re-pin D3 (new `after` after the reviewed native-base change) or to retire the legacy 0941 transition. Then: install Syft 1.55.0 via `install-syft.py`, reuse one `setup-trivy` (`skip-setup-trivy`), drop `download-syft` from the validator inventory (t004), and remove native-base from check 10's exemption so its matrix job gets a timeout.
- Related, outside this milestone: the second history rewrite removed `.pm` evidence directories that native-common qualification reads via `evidence_json`; w8 should confirm the candidate evidence it needs is still reachable.

## Re-triage — 2026-11-12 (w6/119): unchanged

The gate is that `native-base-image.yml` is byte-pinned by w8/m91's D3 custody (`IMAGE_POLICY_FILES` in `.github/scripts/native-common-qualify.py`). It does not depend on which runners bex uses, so the move to GitHub-hosted runners on 2026-11-09 leaves it in place.

## Unblocked — 2026-11-16 (w6/m422)

- **Source:** verified finding. w6/m422 (`e9f27399b`) retired the reviewed-candidate path at the user's 2026-11-16 request. `D3_LEGACY_0941`, `IMAGE_POLICY_FILES` and every validator exemption for the native workflows are gone. Editing `native-base-image.yml` no longer reopens any pinned baseline: a push publishes and qualifies a fresh image, as run [43841677055](https://github.com/bex-co/bex/actions/runs/43841677055) did.
- **Already done by m422:** the matrix job has its check-10 timeout and the file left check 10's exemption. `native-base-image.yml` uses the current setup-buildx and build-push refs.
- **DoD amended:** w8's "125 custody tests" were deleted with the custody chain, so the check is now the native suites.
- **Remaining gates:** none.
- **Next steps:** t002 moves `native-base-image.yml` to `python3 scripts/install-syft.py 1.55.0`, then t003 (one `setup-trivy`, `skip-setup-trivy`) and t004 (drop `download-syft` from the inventory). Accept on the next native-common push run showing the retrying installers.

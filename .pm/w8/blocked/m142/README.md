# w8 · m142 — Clear image-network choices when PORT intent changes

**Worker:** worker1 **Goal:** deleting PORT restores image-port deployment and rollback recovery. **Status:** blocked (t001–t005 and t008 done; t006 hosted acceptance and t007 closeout pending deployment)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Persist configured-port provenance and invalidate mismatches — **DONE** | 30m | — |
| t002 | Recover fresh and rollback networks without inheriting removed PORT — **DONE** | 30m | t001 |
| t008 | Resolve PORT from selected historical inline environment — **DONE** | 30m | t002 |
| t003 | Render parity — **DONE** | 15m | t008 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage — **DONE** | 35m | t003 |
| t006 | Hosted acceptance | 30m | t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- Cache reuse matches configured versus unconfigured intent in both directions, including identical image digests, and unchanged reconciles reuse the result.
- An unknown legacy cache refreshes metadata once; retention cannot copy an old configured primary back into an unconfigured network. Previously served traffic stays on its recorded network until promotion.
- Serve on 80 → fail PORT=9999 → rollback to the release without PORT recovers candidate probes, container PORT and routing on 80. Removing PORT after a successful 9090 release likewise gives a fresh image-default candidate. Explicit 8080 → 9090 still works.
- Private configured-image selection and ordinary native port precedence remain unchanged; historical inline PORT follows the selected release. Status EnvPort reflects the selected release's configuration and clears when PORT is absent.
- Full operator/types tests, real-dependency backend ship gate, workspace lint/dead-code and all three simplify reviews pass.
- Hosted rollback and fresh-deploy controls succeed after deployment, with service continuity checked and fixtures deleted by ID/404 proofs.

## Source + Goal linkage

- **Source:** [w8/210](../../done/210.md), QA sweeps 25/26 on 2026-10-27; promoted 2026-10-28 after current-code reproduction. Earlier HTTP-selection reset in m141 leaves the cached primary and runtime-injected PORT stale.
- **Goal linkage:** Render parity first (ADR008); a failed configuration deploy must have a usable rollback and removing PORT must remove its effect.
- **Expected outcome:** the candidate builds its network from the selected release's port intent, while the existing serving release remains available until recovery is ready.
- **Why now:** the current digest cache only checks configured intent in one direction. Reusing the removed port makes the bug self-sustaining for images that honor PORT.
- **Design:** nullable configured-port provenance (unknown legacy / known unconfigured / exact configured value), symmetric invalidation and conservative legacy HTTP retention. No API/UI schema change is needed; shared runtime status projection reaches REST/GraphQL/MCP/dashboard. Render parity is included for these shared user-visible port and rollback semantics.


## Implementation checkpoint — 2026-10-28

- t001/t002: nullable configured-port provenance, symmetric cache invalidation and conservative legacy retention are implemented. Generated CRD/deepcopy and types tests pass. The controller transition/migration matrix and three API-server recovery specs pass, including fresh and historical-source rollback after failed PORT=9999, and successful 8080→9090→removePORT→80. Existing private/native tests remain in the required full suite.
- Related selection gap found while checking Render parity: Blueprint literals remain in saved `Spec.Env`, but envPort reads that before historical runtime projection. A rollback with saved PORT=9999 can therefore ignore the target's historical absence/8080. Added t008 to select the release's inline configuration without mutating saved settings or changing linked-group rules. This is the same selected-port invariant, not an independent provider feature.


## Local verification — 2026-10-28

Full `make test` (operator unit/envtest, codegen, fmt/vet), `GOWORK=off go test ./...` in types, and `make lint` across all four modules plus whole-program dead-code analysis pass. All three simplify reviews completed; a final Go 1.26 four-case recovery suite passes after reusing the existing probe helper. The old-code overlay reproduces candidate 9090 after successful PORT removal; the saved-Blueprint rollback case independently failed with EnvPort=9999 before t008. Unit tests cover first-pass and materialized historical values, source-record/snapshot cleanup, rollback/current versus restart/historical group rules, and no added ordinary-path record reads. The required real-dependency backend ship gate remains before push. Hosted behavior is not yet claimed.


## Concurrent-fix reconciliation — 2026-10-28

Before this milestone shipped, concurrent `fa918b387` landed a narrower configured-marker cache refusal plus four recovery/control unit cases. The stronger provenance guard supersedes that boolean test while retaining all of its tests. The moved source record retains the concurrent outcome and hosted acceptance gate; there is one owner for remaining acceptance in this milestone. Remote public-Git large-advertisement follow-up and deployment pins are preserved.


## Parked gate — 2026-10-28

Source shipped as `d66f96c06`; the final real-dependency backend gate passed, alongside full operator/types suites, merged recovery controls, all-module lint/dead-code and three simplify reviews. t001–t005 and t008 are complete. **BLOCKED:** the release pipeline must deploy the operator and updated App CRD, then QA runs t006: whoami failed PORT=9999 → rollback on 80, echo 8080 → 9090 → delete PORT → fresh/rollback on 80, and Blueprint saved 9999 → historical rollback on 80 while saved settings stay 9999. Verify runtime PORT, status/probes, explicit/private/native controls and fixture deletion by ID/404 proofs.

The independent [w4/430 live replay](../../../w4/blocked/430.md) on pin `838feec85fdd` still observes an intermittent 4.5 s 502 burst during port promotion. This milestone does not change promotion mechanics or claim that defect fixed. Its zero-error hosted continuity condition also depends on resolving/retesting that upstream item. No production fixture was created by this milestone before its build was available.

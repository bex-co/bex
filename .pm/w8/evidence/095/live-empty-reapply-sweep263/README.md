# Blueprint empty-variable reapply acceptance — sweep263

2026-10-15 UTC, Bex production, isolated human-QA bex-canary workspace. Source `6255ed8b2ca67c8c0f85fd0eadc19bedbbe62459` contains fix06881178a; deployed backend/operator revision unknown. Actual group apply/reapply and native controls pass. Existing [w8/095](../../../done/095.md) is accepted and moved to done; no new issue.

[Complete API-only parser/writer applies, native pre-apply controls, own value reads/revisions and cleanup](owned-api-apply-read-cleanup.json), [24 actual unchanged native multipart/server plan captures](native-plan-captures.json), [declarations and expected identities](cases.json), [versions/coverage](metadata.json), [redactions](redactions.json).

The pinned CLI exposes validation but no apply command. Each apply uses POST /v1/blueprints/deploy with an inline manifest containing only one exact owned environment group. Two real native validations precede every apply. The first successful apply creates the new unlinked group and retains QA_FIRST_EMPTY with an empty value, a harmless retained literal, a nonempty clear-control literal and one generated variable. Its returned ID is immediately recorded, absent from the pre-create metadata inventory. No service/database/Key Value or Blueprint record is created.

| Stage | Actual result | Readback/revision evidence |
| --- | --- | --- |
| First apply, group absent | HTTP200 | empty key present; own literals and generated variable present |
| Reapply with new QA_EMPTY:"" | HTTP200 | new empty key present, ID unchanged, revision advances |
| Repeat the same empty declaration | HTTP200 | all keys retained; revision unchanged |
| Add QA_EMPTY_TWO:"" beside changed QA_KEEP | HTTP200 | both writes appear; omitted keys retained; revision advances |
| Add QA_SPACE:" " and clear QA_TO_EMPTY | HTTP200 | space exact, cleared value present-empty; revision advances |
| Invalid empty variable key | HTTP400 | key set, values, generated fingerprint and revision unchanged |

All eight own value snapshots check exact key presence and every harmless literal. The generated variable is redeclared with generateValue:true in every apply and keeps the same comparison hash. Its actual random value is redacted immediately before any saved read; published evidence contains the placeholder and SHA256 only. These are normal configuration-persistence checks on the unlinked owned group, not a linked process/runtime observation.

Afterward both native clients validate empty, single-space, generated and invalid-key declarations in root, ungrouped and project/environment locations. All24 decisive captured requests pass their expected valid/exit and plan-identity checks: eighteen valid cases remain conservative updates of the same group ID, six invalid cases return validfalse/exit1. Native summaries agree with complete server replies while omitting Bex action extensions. Together with twelve pre-apply commands, there are36 decisive native commands. The read-only matrix leaves the group revision and values unchanged.

One earlier Render/root-space capture is [retained separately](excluded-relay-timeout.json): the owned HTTPS forwarder raised TimeoutError after25seconds before capturing any server response and generated its own502. After a30-second pause, the same read-only validation passes. The relay result is inconclusive and excluded from the24 decisive cases; no product defect is inferred from it.

The existing [focused writer regression](source-test-events.jsonl) passes five behavioral subtests and its parent, including omission and no-op behavior. No product tests are added or full backend suite rerun. Hosted mutating applies are root declarations; the other two declaration locations are native validation coverage and existing shared-parser/writer source coverage. No GraphQL/MCP apply, browser edit, linked workload, internal Secret projection or direct OpenBao inspection is claimed.

The still-unlinked owned group is deleted by exact ID: API204, detail404 and absence from metadata inventory. Baseline group/service IDs remain, and project, Blueprint and quota snapshots equal pre-create values. All native children, forwarding servers and copied configs are reaped/removed. No product fix, commit/push, platform deployment or external message is performed.

Reproduction: [finite owned apply/read/delete driver](fixture-driver.py.txt), [exact-document native capture driver](native-capture-driver.py.txt).

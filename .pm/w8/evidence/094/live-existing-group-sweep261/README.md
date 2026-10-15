# Existing environment-group name acceptance — sweep261

2026-10-15 UTC, Bex production, isolated human-QA workspace bex-canary. Source `6255ed8b2ca67c8c0f85fd0eadc19bedbbe62459` contains fix0e05e9c46; deployed backend/operator revision unknown. All48 actual native validation/capture cases pass. This completes hosted acceptance for existing [w8/094](../../../done/094.md), now moved to done; no new bug.

[Complete native commands/stdout/stderr and unchanged multipart/server replies](native-plan-captures.json), [24 declaration cases and expected identities](cases.json), [owned API-only setup, unchanged readback and cleanup](owned-group-setup-read-delete.json), [versions and coverage](metadata.json), [redactions](redactions.json).

Create exactly one new empty, unlinked environment group through POST /v1/env-groups because this pin has no group-create command. HTTP201 returns its recorded owned ID. The initial detail proves no variables, files, service links or environment assignment; only metadata fields are requested when listing other groups. A fresh exact read after all validation has the same decoded JSON as that initial detail, including its revision. No Blueprint apply or resource reconfiguration occurs.

Published Bex v0.3.2 and unmodified Render v2.27.0 / a764810a768202704e7206eb7b87a47211fcd98e each run all24 declarations: root envVarGroups, ungrouped.envVarGroups and projects[0].environments[0].envVarGroups, with eight name variants per location. Each uses a fresh native process and an exact-document forwarder that permits only POST /v1/blueprints/validate for the intended workspace and manifest bytes. Requests/responses are forwarded unchanged; credentials are omitted only from the recorded copies.

| Variant, each across3 locations ×2 clients | Native result | Complete server-plan result |
| --- | --- | --- |
| Exact existing name | exit0 / validtrue | update, canonical name, same owned ID |
| Leading ASCII, trailing ASCII, surrounding NBSP/EM SPACE | exit0 / validtrue | same update identity; no false create |
| Padded absent name | exit0 / validtrue | create with trimmed canonical name, no existing ID |
| Distinct interior double spaces | exit0 / validtrue | create preserving those interior spaces |
| Whitespace-only name or duplicate after normalization | exit1 / validfalse | existing validation refusal preserved |

Totals are24 correct existing-group updates,12 correct creates and12 invalid-name controls. Every successful group's action preserves its exact declaration sourcePath. Native valid decisions and envGroups summaries agree with the complete response. The pinned decoder still omits Bex mode/actions extensions, so this does not claim the CLI itself displays operation/resourceId; the captured server response proves those fields.

The existing [focused compiler/parser/planner regression](source-test-events.jsonl), TestBlueprintPaddedEnvGroupNamePlansTheExistingGroup, passes on current source, with no new product test or full-suite rerun. Source0e05e9c46 already records the scoped IR seam and inventory/grouping/ownership consumer review. This sweep accepts current-state validation only; hosted apply/reapply, GraphQL/MCP plan displays, browser rendering, Render production and other resource-name normalization remain separate questions.

Exact-ID deletion of the still-empty/unlinked owned group returns204; detail returns404 and metadata inventory omits it while preserving baseline groups. All six baseline services remain. Projects and service/data quotas equal their pre-create snapshots; validation created no project/environment/workload. Internal projection teardown is not inspected. All48 relay servers/threads, native children and copied configs are cleaned. No product changes, commit/push or deployment.

Reproduction: [owned-group driver](fixture-driver.py.txt), [finite native capture and result assertions](native-capture-driver.py.txt).

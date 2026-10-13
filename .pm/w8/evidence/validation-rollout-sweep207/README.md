# Production validation fix acceptance — sweep207

The desired API image advanced to 6ead1ae86285; deployed SHA remains unknown. Both published Bex0.3.2 and unmodified Render2.27/a764810a7682 now observe the previously pending presence, supplied-type and Key Value-property guards. All 104 valid native matrix decisions match current intended behavior; no resource is applied.

- Presence: full 44 previous checks pass. Empty seeded copies validate at root/ungrouped/project locations and in reverse declaration order; omitted prompt copies refuse. Empty and whitespace value/generation contradictions refuse in services and groups; literal/generation/disabled-generation controls pass.
- Types: all 32 prior valid matrix cases pass, including 14 formerly accepted mismatches now refused, 8 matching controls and 10 negative controls. Eight added checks pass for matching worker addressability refusal, static web/static aliases and nested forward references. Some formerly addressability-negative files now correctly refuse earlier on their supplied wrong type; the added correctly typed worker-port cases independently retain the actual addressability guard.
- Key Value: keyvalue/redis envVarKey-only refs now refuse; host/connectionString controls validate. Public guidance still lists password even though the manifest schema excludes it, so w8/098's active guidance work remains.

Five exact native multipart captures confirm corrected empty-seed acceptance, generated-empty refusal, supplied-type refusal and Key Value refusal/control. All HTTP200 and native decoded validity/errors equal API results. Harmless bytes and own ownerId pass unchanged; Authorization omitted. Eight initial KV schema refusals lacked required ipAllowList and are excluded from guard claims. Corrected harmless declarations use explicit internal-only empty allowlist. The original driver snapshot contains that incomplete initial KV setup; complete fixtures and archived corrected results are authoritative.

The reviewed source trees for apps/deploys/core/controller/types/CLI remain byte-identical between recorded c5a0eb9dd15f and verified 3c6809182. This confirms the prior source diagnoses while avoiding an assumed deployed SHA. A concurrent shared actor committed earlier PM filings; this QA process did not commit/push. Actual HEAD during each earlier diagnostic was not separately captured; the tree identity record states this limit.

No hosting resource, generated runtime value, project, SQL session or paid feature is created. Eligible worker plans appear only in validation documents and are never provisioned. Native comparison/capture configs, servers and children are removed. Quota remains six baseline services, zero terminating and no datastores. Hosted apply/re-sync, live seed retention, GraphQL/MCP/dashboard and outside-file host checks remain untested and cannot be marked complete from validation alone.

Evidence:

- [environment-group-generated-plus-empty.yaml](environment-group-generated-plus-empty.yaml)
- [envrefs.json](envrefs.json)
- [generation-driver.py.txt](generation-driver.py.txt)
- [generation_forms.json](generation_forms.json)
- [initial-rollout-driver.py.txt](initial-rollout-driver.py.txt)
- [kv-connection-control.yaml](kv-connection-control.yaml)
- [kv-env-only.yaml](kv-env-only.yaml)
- [kv-host-control.yaml](kv-host-control.yaml)
- [metadata-cleanup.json](metadata-cleanup.json)
- [native-wire.json](native-wire.json)
- [project-seed-empty.yaml](project-seed-empty.yaml)
- [redis-env-only.yaml](redis-env-only.yaml)
- [reference_types.json](reference_types.json)
- [rollout-capture-driver.py.txt](rollout-capture-driver.py.txt)
- [root-seed-empty.yaml](root-seed-empty.yaml)
- [seed-capture-driver.py.txt](seed-capture-driver.py.txt)
- [seed-driver.py.txt](seed-driver.py.txt)
- [service-generated-plus-empty.yaml](service-generated-plus-empty.yaml)
- [source-identities.json](source-identities.json)
- [type-actual-worker-addressability.yaml](type-actual-worker-addressability.yaml)
- [type-capture-driver.py.txt](type-capture-driver.py.txt)
- [type-port-pserv.yaml](type-port-pserv.yaml)
- [type-project-forward-reference.yaml](type-project-forward-reference.yaml)
- [type-static-static-alias.yaml](type-static-static-alias.yaml)
- [validation_rollout.json](validation_rollout.json)

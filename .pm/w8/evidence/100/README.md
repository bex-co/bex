# Pooled-reference diagnostic evidence — sweep194

On 2026-10-13 UTC, both native clients reject a same-file `connectionPoolString` reference to a database without pooling, but report a nonexistent service `connectionPool` field and the service header's location. Eight matrix cases cover root/project nesting and first/second env entries; two extra exact native captures reproduce the wrong API path. Two captured direct-connection controls pass.

The 44-process matrix has 40 correctly checked admission decisions, including all Free capacity controls, plus four excluded QA fixture errors caused by an overlong direct-control database name. Those four refusals are retained; shortened controls pass in both clients/layouts. No paid feature is applied. The bug is diagnostic association, not acceptance or admission.

Four virtual current-source `ValidateBlueprint` cases reproduce the absent path and wrong line. These diagnostic assertions expect the current bug; they are not fixed-source regression tests. Existing nearby capacity/location tests pass. No full backend suite is claimed. Native wire captures preserve the exact multipart POST to the Bex endpoint and HTTP200 result; Authorization is omitted and only non-secret headers retained. Non-intercepted matrix processes are distinguished from the four captured processes.

Every copied native capture config/server/thread is removed. No hosting resource, group, SQL connection, pooler or replica is created. Quota stays at six services and no terminating/datastore resources. Source revision c5a0eb9dd15f6b417bf586fdf26510706497b032; deployed revision unknown. Known credential/email values and private paths are redacted. No product implementation, commit or push is performed.

Evidence:

- [callers-dedupe.json](callers-dedupe.json)
- [corrected-control-driver.py.txt](corrected-control-driver.py.txt)
- [existing-neighbor-events.jsonl](existing-neighbor-events.jsonl)
- [existing-neighbor-tests.json](existing-neighbor-tests.json)
- [location-events.jsonl](location-events.jsonl)
- [location-input.json](location-input.json)
- [location-overlay.json](location-overlay.json)
- [location-probe_test.go.txt](location-probe_test.go.txt)
- [location-results.json](location-results.json)
- [matrix-driver.py.txt](matrix-driver.py.txt)
- [metadata-cleanup.json](metadata-cleanup.json)
- [native-capture-driver.py.txt](native-capture-driver.py.txt)
- [native-matrix.json](native-matrix.json)
- [native-wire.json](native-wire.json)
- [project-direct-reference.yaml](project-direct-reference.yaml)
- [project-pool-reference.yaml](project-pool-reference.yaml)
- [project-second-pool-reference.yaml](project-second-pool-reference.yaml)
- [root-direct-reference.yaml](root-direct-reference.yaml)
- [root-pool-reference.yaml](root-pool-reference.yaml)
- [root-second-pool-reference.yaml](root-second-pool-reference.yaml)
- [second-entry-driver.py.txt](second-entry-driver.py.txt)
- [source.txt](source.txt)

# Indexed static rule errors — sweep202

Both published Bex0.3.2 and unmodified Render2.27/a764810a7682 correctly reject invalid second static route/header entries, but report the service declaration or its unrelated name. Root and project-environment placements each reproduce four shapes in both clients: 16 wrong locations. Combined valid rules, a normal splat redirect, empty header value and empty lists provide 16 passing controls. No rules are applied.

Four exact native multipart captures prove two incorrect API diagnostic replies and two valid controls. All are POST /v1/blueprints/validate, HTTP200; the owner and harmless file bytes pass unchanged, Authorization is omitted, and native validity/errors equal the API response. Eight actual current-source ValidateBlueprint diagnostics reproduce the wrong line/path and show the desired field exists in source.Locations. These diagnostics assert the current bug. Four neighboring existing tests pass; this is not a full backend suite or a fixed-source test.

The earlier virtual diagnostic input omitted the # pointer prefix, so that QA attempt failed its own location lookup. It is retained separately and excluded from product claims. The corrected input passes all eight diagnostic cases.

No hosted resource or paid feature was created. All capture processes, servers, copied configs and comparison config are removed. Six baseline services remain, zero terminating and no datastores. Current checkout c5a0eb9dd15f6b417bf586fdf26510706497b032; live revision unknown. Credentials/email and private paths are redacted. Product implementation, commits and pushes are outside this filing.

Evidence:

- [excluded-diagnostic-fixture-attempt.json](excluded-diagnostic-fixture-attempt.json)
- [location-events.jsonl](location-events.jsonl)
- [location-input.json](location-input.json)
- [location-overlay.json](location-overlay.json)
- [location-probe_test.go.txt](location-probe_test.go.txt)
- [location-results.json](location-results.json)
- [metadata-cleanup.json](metadata-cleanup.json)
- [native-capture-driver.py.txt](native-capture-driver.py.txt)
- [native-driver.py.txt](native-driver.py.txt)
- [native-matrix.json](native-matrix.json)
- [native-wire.json](native-wire.json)
- [project-combined-valid.yaml](project-combined-valid.yaml)
- [project-empty-header-value.yaml](project-empty-header-value.yaml)
- [project-empty-lists.yaml](project-empty-lists.yaml)
- [project-second-header-name.yaml](project-second-header-name.yaml)
- [project-second-header-path.yaml](project-second-header-path.yaml)
- [project-second-route-destination.yaml](project-second-route-destination.yaml)
- [project-second-route-source.yaml](project-second-route-source.yaml)
- [project-splat-redirect.yaml](project-splat-redirect.yaml)
- [root-combined-valid.yaml](root-combined-valid.yaml)
- [root-empty-header-value.yaml](root-empty-header-value.yaml)
- [root-empty-lists.yaml](root-empty-lists.yaml)
- [root-second-header-name.yaml](root-second-header-name.yaml)
- [root-second-header-path.yaml](root-second-header-path.yaml)
- [root-second-route-destination.yaml](root-second-route-destination.yaml)
- [root-second-route-source.yaml](root-second-route-source.yaml)
- [root-splat-redirect.yaml](root-splat-redirect.yaml)
- [source.txt](source.txt)

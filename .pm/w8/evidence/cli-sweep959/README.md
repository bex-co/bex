# CLI sweep 959 — restart/scale body discipline and plan gate (live, 2026-10-20)

Two disposable free web services created, exercised, deleted (204s; GETs 404; name list empty). No new bug filed.

- `POST /services/{id}/restart` with an empty JSON body → coded 400 `request body is not allowed for this operation`; with no body at all → 200 returning the service object. The no-body operations are strict about the contract, coded either way.
- `POST /scale {numInstances:2}` on a free service → coded 400 naming the gate: `the free plan is limited to 1 instance(s) and cannot scale to 2` (fail-closed refusal; no paid transition attempted). `numInstances:0` and a string value → coded 400 `must be 1-100`.
- Suspend/resume with no body → 202 each, on the second fixture.

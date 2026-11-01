# m83 actual deployment-source gate

The one read-only inventory run[41251157373](https://github.com/bex-co/bex/actions/runs/41251157373) completed successfully. Its actual23:30:23–23:30:34UTC observations show manager generation1702 with one Ready replica and API generation2818 with two Ready replicas. All three active pods match their immutable spec/runtime image references. Fresh strict Cosign3.1.4 verification also succeeds for the exact deployed image and source.

Admission fails on the actual operator source: the deployed b0ecf831… image is signed from`d147703d7657f8795593deb2e74c9e7eac426cd2`. That source lacks edge_compression.go and has the older app_controller.go; required fix`a15472735e63d9217b4db49f873ceaa69324f5d3` is not its ancestor. Healthy pods and a valid signature do not establish this fix's deployment. No tenant login or fixture was started.

The exact live proxy-version prerequisite came from the private plan. The dated scope audit cites the user's gzip-only decision and required actual wire/stream/source/cleanup behavior; it records actual proxy version UNKNOWN without inferring it from a checked-in pin. All mandatory functional and signed operator-source gates remain.

One bounded release triage found no proven CI defect: the active ea5a901da build passed its required tests and five-image pipeline, then entered deployment checkout; that source also predates the compression fix. The fix-containing99cdc run was pending with no jobs. The inspected canceled a154/c680 runs had no build jobs or qualified candidates. No cancellation, rerun or pin change was issued.

[CUSTODY.json](CUSTODY.json) preserves the projected platform evidence, actual signature/source results, raw verification outputs and scope/release reviews as gzip. The original full inventory and job log remain private with exact hashes in[PRIVATE-INPUT-CUSTODY.json](PRIVATE-INPUT-CUSTODY.json). A newly rolled-out signed operator containing the exact fix, followed by fresh admissible readiness evidence and legal Free-budget preflight, is required before live acceptance.

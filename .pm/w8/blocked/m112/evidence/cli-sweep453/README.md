# Native TimescaleDB migration and Apache hypertable reference

Sweep453 reproduces missing TimescaleDB delivery through both unchanged native clients against one fresh Free PostgreSQL17.11 database. Native create exits0, both ordinary connections pass, and both CREATE EXTENSION IF NOT EXISTS migrations exit1 with a server-reported missing control file. Four before/after catalogs are identical and omit TimescaleDB. Two ordinary own-table transactions match eight independent UTC initial/insert/update/delete snapshots, including Unicode/signed values and times on both sides of the Unix epoch; all own schema/data changes roll back.

The real local immutable official image runs TimescaleDB2.22.1/PG17.6 on arm64 with actual Apache mode. A normal local database owner enables the extension, creates a real hypertable with four chunks/four physical relations, and obtains all four independently expected time_bucket snapshots. Own schema/hypertable metadata is absent after rollback. No Community functions, performance, all-major compatibility or production CNPG supplier is claimed. Image digest/module version and the upstream release source tag are separate provenance.

Two excluded local harness trials are retained: an unnecessary final setting read and a connection to the temporary initialization server during its shutdown. The final harness removes that read and requires final Postgres PID1 plus a ready socket; all consumer checks pass. All three own containers/anonymous volumes/contexts are removed, with the public base image cache preserved.

Native deletion exits0; detail404/listabsence, cached SQL rejection, private credential/CA removal, baseline/current inventory and termination0 verify hosted cleanup. Only the continuing isolated QA login remains. Native create/SQL wire and actual serving platform SHA/OCI/config are unobserved. No product implementation or ship; this newly researched major filing is uncommitted.

- [Measured results](results.json)
- [Complete native catalogs, controls and cleanup](complete-native-timeseries-catalogs-controls-and-cleanup.json)
- [Actual local Apache-mode consumer](actual-local-Apache-hypertable-results.json)
- [Exact SQL and independent UTC model](exact-SQL-and-independent-UTC-bucket-DML-model.py.txt)
- [Source, pinned consumer, callers and dedupe](contract-dedupe-and-source-research.json)
- [Pinned primary source](pinned-primary-package-research.json)
- [Primary Apache/restore references; restore unrun](official-Apache-and-lifecycle-reference.json)
- [Excluded diagnostic read](excluded-local-final-diagnostic-setting-read.json)
- [Excluded initialization-server race](excluded-local-initialization-server-race.json)
- [Final inventory](final-inventory.json)

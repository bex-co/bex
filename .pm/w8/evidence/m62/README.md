# Environment values containing NUL: validation and consumer evidence

The [12 native validation receipts](native.json) show Bex v0.3.2 and unmodified same-pin Render v2.27.0 accepting NUL values in service literals, `sync:false` seeds and group literals. All six newline controls also pass. Native multipart traffic was not intercepted; source tracing and the exact harmless documents are retained.

- Actual [typed parser observations](parser.json), [inputs](parser-input.json), [diagnostic](parser-probe_test.go.txt) and [virtual overlay](parser-overlay.json): all six values survive unchanged.
- Actual [shared patch observations](core.json), [diagnostic](core-probe_test.go.txt) and [overlay](core-overlay.json): NUL, newline, empty, Unicode and numeric-string values are preserved by the current implementation.
- [Local OCI runtime](local-env.json): ordinary 18-byte value starts successfully; the 18-byte NUL value is refused before the programme starts. Exact stderr, IDs, create proofs and cleanup are retained.
- [Actual generated-loader execution](local-loader.json) and [verbatim loader](generated-loader.sh.txt): ordinary 18-byte value stays exact; the NUL value becomes 17 bytes, with a shell warning. This is a local execution, not a hosted build.
- [Binary file control](local-file.json): the same NUL-containing 18 bytes and hash survive a mounted file exactly. Environment-value rejection must not reject file content.
- [Source](source.txt), [counted callers](callers.json), [metadata](metadata.json) and [contract research](contract-research.json).
- Tested historical drivers: [native](native-driver.py.txt), [runtime](local-env-driver.py.txt), [file](local-file-driver.py.txt), [loader](local-loader-driver.py.txt). These use `<QA_TMP>` / `<REPO>` placeholders and depend on the hunt's isolated harness; authenticate independently before adapting them.

No hosted service or environment group was created in this sweep. Five fresh local containers were created by ID-backed ownership proofs, removed by their exact IDs and confirmed absent. The cached image remains. The Go overlays are diagnostics of current behavior, not a fix or full backend suite. Live API writes, Blueprint apply and production NUL runtime were not tested. Deployed revision is unknown; source diagnosis is at 97ce481c8eabb13025c118ea9fa96d0aa83832d3. No product code was changed.

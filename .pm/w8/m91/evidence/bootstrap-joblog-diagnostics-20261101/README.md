# Recover qualification setup diagnostics when artifact upload fails

Actual fb73 run41280688171/job124773320642 fails all three bounded BuildKit pulls before runtime tests. Cleanup fails too; upload then loses its connection, leaving the detailed pull errors unavailable. [Full failure custody](RAW-CUSTODY.json) retains the exact job log and complete metadata without claiming a runtime regression or successful baseline.

The narrow helper change mirrors lossless bounded pull bytes/status/SHA into job logs and records fixed owned cleanup results. Auth contents and paths stay excluded. Commands, pins, retry/time/resource bounds, ownership, cleanup errors and qualification gates remain unchanged. The underlying pull cause is still unknown.

All83 host controls pass for the author, independent reviewer and Root. New regressions fail on the old helper and recover exact quoted/Unicode/NUL/non-UTF8 bytes after the artifact channel is removed. Root image-only planning preserves the same approved56c58 supplier; family reuse still refuses its missing complete baseline. Root’s initial reader-schema and leaked plan-environment test errors are retained with their corrections; no source or assertion was relaxed.

The old94cf arm64 cohort independently confirms the same previously repaired setuptools diagnostic mismatch: all four positive cases pass, with owned cleanup and upload successful. Its full results remain historical. The predeploy-A refresh changes only four approved current input identities and the derived observer-command hash; independent source review is clean. No phase/image/Job was executed.

Fresh full successful utility artifacts on both executing architectures, then the original35 family cases, same-common predeploy, adoption and original hosted acceptance remain open.

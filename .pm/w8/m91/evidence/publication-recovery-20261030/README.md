# Native publication recovery checkpoint

The older `bfdeb622…` matrix built Node 14/16 candidates and passed their real architecture probes, then Trivy v0.70.0 exhausted its default 300-second analysis deadline on GCC `lto1`. Those candidates remain unapproved. Commit `059c163b3` sets the existing timeout input to fifteen minutes on both scans; architecture/scanners/reports/probes/signatures and runners are unchanged. Workflow validation and actionlint pass. The new matrix is queued; successful recovery is not yet established.

Node 18 failed separately at GHCR login, with a token-header transport timeout before build. A single job-only rerun was rejected; the original failed attempt remains unchanged. No running compile was cancelled. Full compressed job logs, exact excerpts, read-only reconciliation and the write response are retained with raw/compressed hashes.

Candidate inline cache metadata exists on both architectures, but no compiler reuse was measured. It is not a signed or gated supplier, and no discovery tag or production pin was promoted. Root continues to require all twenty-seven reviewed multiarch candidates and original hosted acceptance before adoption/closure.

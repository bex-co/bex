# Authenticate common-image generated qualification

The old common workflow's ARM64 generated-image builds failed before functional commands because GHCR denied the canonical published common image's token request. Its AMD64 leg separately stopped in Buildx bootstrap on a Docker Hub TLS handshake timeout. These raw failures are retained separately; neither is a runtime/addon acceptance result.

The qualification job now requests packages:read and logs in to GHCR with its job token through bounded password-stdin. Before login, initialization records ownership and creates a mode0700 Docker-config sibling outside the uploaded evidence root. Subsequent Docker/Buildx commands inherit this exact config. Final cleanup removes the exact owned builder/resources and attempts quiet logout and credential-tree removal even after earlier cleanup errors; config contents and logout output are excluded from evidence. Foreign, symlinked or mismatched scopes are refused. Original publication, runtime, scan, signature, report, source and architecture gates remain in place.

Fourteen host controls pass, covering login argument/stdin/environment flow, scope/mode, partial and late initialization failures, cleanup failure paths and ownership refusals. Workflow lint passes with only the exact pre-existing custom bex-ci runner-label diagnostic accounted for. Independent review is being finalized at this checkpoint. No actual GHCR login, Docker or CI-state mutation was performed for these source checks; a fresh workflow must establish the generated-image result.

[CUSTODY.json](CUSTODY.json) retains exact source bytes, tests, workflow checks and both original failed-job logs as gzip. This fixes the authenticated qualification setup within[m91/t007](../../t007.md); it does not fill a production pin.


## Independent review — 2026-10-31

CLEAN. All frozen payloads and three live source files match. The independent14-test host rerun passes, plus an extra actual-filesystem/stub-process probe: a failed environment export followed by builder exit17 still removes the exact credential tree, preserves the failure and leaves foreign config untouched. Workflow lint and diff checks pass. Actual authenticated generated-image execution remains the next gate.

# w2/m167 local CLI and transport acceptance — 2026-10-02

The installed Bex v0.2.1 and unchanged upstream Render v2.27.0 pin each returned both `qa_text_A` and `qa_text_B` exactly once in both `--text qa_text_A,qa_text_B` and reversed order, excluding the unrelated control. All four commands exited 0 with empty stderr.

## Substrate

An owned Alpine 3.20 container emitted separate A, B, unrelated-control, and literal `[a].*` lines to stdout. The harness read that actual Docker stdout and ingested it with production-shaped namespace/app/container/type/level labels into an isolated Docker Loki 3.6.1. The current composed Bex API queried that real Loki via `logs.NewLokiSource`; no response stubs were used for log history or CLI responses. The App ID was minted by `internal/id` and seeded into an in-memory Kubernetes client. The standard test introspection fixture authenticated a local token, with the store-off default workspace.

This is local process acceptance, not production deployment or operator provisioning. Kubernetes reconciliation, Alloy shipping, real OAuth/OpenFGA, managed database servers, pod-log fallback and live build/app tails were not exercised here; automated backend regressions cover the latter source/filter boundaries. The emitter-to-Loki hop was explicit harness ingestion, not an installed log shipper. No production credentials or services were used. The shared development cluster was not changed; m166 already documents its broken provisioning substrate.

The unchanged Render binary was built directly from module cache pin `a764810a768202704e7206eb7b87a47211fcd98e`, using the upstream release's version linker flag. No CLI source was modified. Binary hashes and versions are recorded in `versions.json`.

## Results

`local-acceptance.json` contains 17 sanitized request/command captures:

- Four installed Bex / unchanged Render commands, with both argument orders, each returned A and B once.
- REST repeated and reversed terms returned the same union; duplicate terms returned A once; uppercase scalar matched A; `[a].*` matched literally; no-match returned no rows.
- GraphQL retained its scalar `text` argument and case-insensitive A match.
- MCP initialization succeeded and `list_logs` with reversed array terms returned A and B.
- OR text terms combined with nonmatching `level=error` returned no rows. An empty term beside A did not widen the query.
- Two `limit=1` backward pages returned B then A, once each, using the first response's `nextEndTime`.

Initial harness setup attempts omitted a CLI workspace and collided a config-directory name with the Render binary path. Those commands failed before querying logs; setting the isolated config directories and store-off workspace fixed setup. Final captures are the successful rerun against the final implementation.

## Cleanup

The temporary API listener and introspection server exited cleanly. The temporary Go test harness was removed. Both owned Docker containers and their anonymous volumes were removed, deleting Loki fixture storage and emitter stdout. Private CLI configuration and temporary scripts containing the test-only token were removed. Only sanitized results, binary provenance, and this report remain in the repository.

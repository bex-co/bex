# w6 · m163 — Native image workflows install Syft and Trivy with retries

**Worker:** worker6 **Goal:** no workflow depends on a single-attempt Syft or Trivy download. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [`install-syft.py` takes a version; reviewed sha256s for 1.51.1 and 1.55.0](t001.md) | 40m | — |
| t002 | [pg-operand, native-base and native-common use `install-syft.py 1.55.0`](t002.md) | 50m | t001 |
| t003 | [native-base installs Trivy once and scans with `skip-setup-trivy`](t003.md) | 30m | — |
| t004 | [Drop download-syft from the reviewed action inventory](t004.md) | 15m | t002 |
| t005 | [Simplify](t005.md) | 15m | t003, t004 |
| t006 | [Test coverage](t006.md) | 20m | t005 |
| t007 | [Closeout](t007.md) | 10m | t006 |

## Definition of done

- `git grep download-syft .github` is empty; the validator, its self-test and w8's 125 custody tests pass.
- The next run of each changed workflow shows the retrying installers in its log.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w6` 2026-11-06, proposal 5; the 2026-11-05 Syft diagnosis (Option B) and native-common run 42037362778.
- **Goal linkage:** ADR008 reliable delivery: image publication should not fail on one dropped download.
- **Expected outcome:** native and operand image builds survive transient GitHub download failures.
- **Why now:** the host link is degraded now, and these single-attempt installers are the next reds. Touches w8-pinned files: coordinate in the commit and notify w8.
- **Render parity:** omitted — CI/infrastructure only, no REST/GraphQL/MCP/UI surface.

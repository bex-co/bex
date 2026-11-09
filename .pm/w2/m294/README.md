# w2 · m294 — dev-N publishes and serves static sites (local object store)

**Worker:** worker2 **Goal:** on the local mock, a static site publishes to a local S3-compatible bucket and is served through Traefik, and its delete purges the bucket prefix, so static sites can be walked end to end on dev-N. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `mock-cluster.sh` installs a pinned local S3-compatible store (MinIO, sha-checked) with a `bex-static` bucket and dev credentials (never printed) | 1h | — |
| t002 | `mock-cluster.sh operator` points the operator's static store at it (mock-only override) and creates `bex-static-publish-s3` plus the static-server's Secrets | 1h | t001 |
| t003 | dev-2 walk: `examples/static-site` publishes and serves 200 through Traefik; delete purges its prefix and finishes unattended | 45m | t002 |
| t004 | Simplify                                                                                  | 15m | t003 |
| t005 | Test coverage                                                                             | 30m | t004 |
| t006 | Closeout                                                                                  | 10m | t005 |

## Definition of done

- On a fresh mock, a dev-2 static site created from `https://github.com/bex-co/bex` `examples/static-site` goes Live and its page is served (200, expected body) through Traefik.
- Deleting it removes its objects from the local bucket, and its App CR goes away without manual edits.
- Production manifests and behavior are unchanged: the Wasabi endpoint and the Secret names stay as they are.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w2`, 2026-11-09, item 1, from the w2/076 and w2/m293 dev-2 walks (2026-11-08).
  - Static sites failed with `StaticCredentialUnavailable` (`bex-build/bex-static-publish-s3` missing).
  - The operator's store points at real Wasabi (`s3.eu-central-2.wasabisys.com`).
  - The mock's `bex-static-server` is 0/2 for want of platform Secrets.
- **Goal linkage:** static sites are a core Render resource type (ADR029); dev-N is every workstream's verification environment.
- **Expected outcome:** static-site changes can be walked locally instead of only on production.
- **Why now:** m289 and m293 made web services and repo builds work locally. Static sites are the last core type dev-N cannot publish.
- **Render parity omitted:** local tooling only.

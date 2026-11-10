# w2 · m294 — dev-N publishes and serves static sites (local object store)

**Worker:** worker2 **Goal:** on the local mock, a static site publishes to a local S3-compatible bucket and is served through Traefik, and its delete purges the bucket prefix, so static sites can be walked end to end on dev-N. **Status:** done (2026-11-09)

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `mock-cluster.sh` installs a pinned local S3-compatible store (MinIO, sha-checked) with a `bex-static` bucket and dev credentials (never printed) — **DONE** | 1h | — |
| t002 | `mock-cluster.sh operator` points the operator's static store at it (mock-only override) and creates `bex-static-publish-s3` plus the static-server's Secrets — **DONE** | 1h | t001 |
| t003 | dev-2 walk: `examples/static-site` publishes and serves 200 through Traefik; delete purges its prefix and finishes unattended — **DONE** | 45m | t002 |
| t004 | Simplify — **DONE**                                                                                  | 15m | t003 |
| t005 | Test coverage — **DONE**                                                                             | 30m | t004 |
| t006 | Closeout — **DONE**                                                                                  | 10m | t005 |

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

## Closed — 2026-11-09 (`/loopx w2`)

- **Store:** MinIO (`docker.io/bitnamilegacy/minio:2025.7.23-debian-12-r5@sha256:6dabb4a2…`, digest-pinned, multi-arch; the community images were withdrawn upstream) runs in `bex-static-store` with bucket `bex-static`.
  - Three random credentials: root, a publish user in `bex-build/bex-static-publish-s3`, and a read-only user in `bex-system/bex-static-read-s3`. Each policy is scoped to the bucket. They are never printed and are kept across runs.
  - A mock-only NetworkPolicy lets only publish and purge pods reach MinIO.
- **Operator overrides (mock only):** `mock-cluster.sh operator` sets `BEX_STATIC_S3_ENDPOINT/BUCKET/REGION` and creates `bex-static-config`. `bex-static-server` went from 0/2 to 2/2. Production's Wasabi values are untouched, and a test asserts that.
- **dev-2 walk:** a static site from `examples/static-site` reached Running (Published, Routed) in 2m44s.
  - The bucket held `rev-1/index.html`. Through Traefik with the bare Host, the page returned **200**, byte-identical to `examples/static-site/index.html`.
  - DELETE returned 204, the purge Job completed, and the App CR was gone in 9 s with no manual edits. The bucket was then empty.
- **Checks:** contract tests pass; deliberately breaking 8 behaviours made each one fail. shellcheck is clean.

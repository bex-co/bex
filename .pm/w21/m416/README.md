# w21 · m416 — wrangler's `r2` contract over the same store: buckets, streamed objects, CORS, lifecycle, the sweep, purge and shadow meters

**Worker:** worker21 **Goal:** stock `wrangler r2` with three environment variables manages buckets and objects in bex, and the usage, lifecycle and cleanup machinery behind both contracts runs. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The Cloudflare envelope, error table, route mount and authentication under `/cloudflare/client/v4` | 50m | w21/m415/t005 |
| t002 | Bucket routes: create, list, info and delete, location hints, and the `getR2StorageMetrics` answer | 50m | t001 |
| t003 | Object put streams through bex-api: length required, 300 MiB cap, extended deadlines, quota gate and concurrency cap | 60m | t001, w21/m415/t005 |
| t004 | Object get and delete: head first, stream back with the stored headers | 40m | t003 |
| t005 | CORS and lifecycle routes: store the rules on the bucket row and validate them | 45m | t002 |
| t006 | The not-offered Cloudflare surface, the scope matrix entries and the authorization matrix | 45m | t002, t003, t004, t005 |
| t007 | wrangler contract tests: a recorded-fixture Go test, a pinned real-wrangler script and a weekly workflow | 60m | t005, t006 |
| t008 | The hourly sweep: per-bucket usage from the prefix listing | 50m | w21/m415/t001, w21/m415/t003, t002 |
| t009 | Lifecycle execution inside the sweep | 45m | t005, t008 |
| t010 | Delete a workspace or account: purge `<tea-id>/` in both regions, drop the bucket rows, retry from the sweep | 50m | t008 |
| t011 | Shadow meters: storage GB-seconds, ingress bytes and egress bytes at $0 | 45m | t008, t003, t004 |
| t012 | Docs: the `bex-cli.md` wrangler section, the ADR018 Extension row and dated divergences, the AGENTS inventories | 40m | t007, t010, t011 |
| t013 | Production walk for Contract B: real wrangler against production, a 300 MiB upload, Render and wrangler on the same data | 60m | w21/m415/t010, t012 |
| t014 | Render parity (Cloudflare R2 reference for the Cloudflare-shaped surface) | 30m | t013 |
| t015 | Simplify | 20m | t014 |
| t016 | Test coverage | 45m | t014, t015 |
| t017 | Closeout | 10m | t016 |

## Definition of done

1. Stock wrangler 4.162.0 with `CLOUDFLARE_API_BASE_URL`, `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` runs bucket create, list, info and delete, object put, get and delete with `--remote`, `cors list|set|delete` and `lifecycle list|add|set|remove` against bex.
2. A 300 MiB upload completes through Traefik, 300 MiB + 1 is refused, and an object put with wrangler into `render-<region>` is readable with `render ea objects`.
3. The hourly sweep writes per-bucket usage, executes lifecycle rules (at most 10,000 deletions per rule per sweep) and retries purges; deleting a workspace or account leaves no object or bucket row.
4. The three shadow meters appear in usage at $0 and the Stripe items are unchanged.
5. The contract tests (fixtures and the pinned real-wrangler script) run in CI weekly, and the production walk has recorded the Wasabi streamed-PUT result.

## Source + Goal linkage

- **Source:** [ADR112](../../../docs/ADR112-object-storage.md), filed by `/pmx adr 112 to w21` on 2026-11-15; ADR112 tasks t003, t005, t007 and the Contract B parts of t008 and t009, reorganized into three milestones.
- **Goal linkage:** ADR008 Render parity first, and the user's 2026-11-14 goal: serve customers' object needs while Render's early-access contract has no bucket, CORS, lifecycle or public access, by also serving the contract of `wrangler r2`.
- **Expected outcome:** A customer manages buckets, CORS and lifecycle with a tool they already have, and the platform meters and cleans up what they store.
- **Why now:** Needs the registry and client from w21/m415; the sweep and purge are what keep the store from accruing cost for deleted workspaces.
- **Render parity task:** included. The Cloudflare-shaped REST surface is customer-facing; the reference product is Cloudflare R2 and wrangler (Render has none), and the shared-data check keeps the Render contract true.

## Open decisions

- Recorded recommendation, not a user decision: every bex-chosen value (caps, error-code subset, key rule, regions) is unsigned in ADR018 until the user signs it.
- Recorded recommendation: if Wasabi rejects a streamed presigned PUT, fall back to a single `PutObject` with the known length rather than spooling.
- Not filed: ADR112 t010 (pricing) and the S3 API with tenant credentials (D10); both wait for the user.

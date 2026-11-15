# w21 · m416 — wrangler's `r2` contract over the same store: buckets, streamed objects, CORS, lifecycle, the sweep, purge and shadow meters

**Worker:** worker21 **Goal:** stock `wrangler r2` with three environment variables manages buckets and objects in bex, and the usage, lifecycle and cleanup machinery behind both contracts runs. **Status:** done (2026-11-15)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The Cloudflare envelope, error table, route mount and authentication under `/cloudflare/client/v4` — **DONE** | 50m | w21/m415/t005 |
| [t002](done/t002.md) | Bucket routes: create, list, info and delete, location hints, and the `getR2StorageMetrics` answer — **DONE** | 50m | t001 |
| [t003](done/t003.md) | Object put streams through bex-api: length required, 300 MiB cap, extended deadlines, quota gate and concurrency cap — **DONE** | 60m | t001, w21/m415/t005 |
| [t004](done/t004.md) | Object get and delete: head first, stream back with the stored headers — **DONE** | 40m | t003 |
| [t005](done/t005.md) | CORS and lifecycle routes: store the rules on the bucket row and validate them — **DONE** | 45m | t002 |
| [t006](done/t006.md) | The not-offered Cloudflare surface, the scope matrix entries and the authorization matrix — **DONE** | 45m | t002, t003, t004, t005 |
| [t007](done/t007.md) | wrangler contract tests: a recorded-fixture Go test, a pinned real-wrangler script and a weekly workflow — **DONE** | 60m | t005, t006 |
| [t008](done/t008.md) | The hourly sweep: per-bucket usage from the prefix listing — **DONE** | 50m | w21/m415/t001, w21/m415/t003, t002 |
| [t009](done/t009.md) | Lifecycle execution inside the sweep — **DONE** | 45m | t005, t008 |
| [t010](done/t010.md) | Delete a workspace or account: purge `<tea-id>/` in both regions, drop the bucket rows, retry from the sweep — **DONE** | 50m | t008 |
| [t011](done/t011.md) | Shadow meters: storage GB-seconds, ingress bytes and egress bytes at $0 — **DONE** | 45m | t008, t003, t004 |
| [t012](done/t012.md) | Docs: the `bex-cli.md` wrangler section, the ADR018 Extension row and dated divergences, the AGENTS inventories — **DONE** | 40m | t007, t010, t011 |
| [t013](done/t013.md) | Production walk for Contract B: real wrangler against production, a 300 MiB upload, Render and wrangler on the same data — **DONE** | 60m | w21/m415/t010, t012 |
| [t014](done/t014.md) | Render parity (Cloudflare R2 reference for the Cloudflare-shaped surface) — **DONE** | 30m | t012 |
| [t015](done/t015.md) | Simplify — **DONE** | 20m | t014 |
| [t016](done/t016.md) | Test coverage — **DONE** | 45m | t014, t015 |
| [t017](done/t017.md) | Closeout — **DONE** | 10m | t016 |

## Definition of done

1. Stock wrangler 4.162.0 with `CLOUDFLARE_API_BASE_URL`, `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` runs bucket create, list, info and delete, object put, get and delete with `--remote`, `cors list|set|delete` and `lifecycle list|add|set|remove` against bex.
2. A 300 MiB upload completes through Traefik, 300 MiB + 1 is refused, and an object put with wrangler into `render-<region>` is readable with `render ea objects`.
3. The hourly sweep writes per-bucket usage, executes lifecycle rules (at most 10,000 deletions per rule per sweep) and retries purges; deleting a workspace or account leaves no object or bucket row.
4. The three shadow meters appear in usage at $0 and the Stripe items are unchanged.
5. The contract tests (fixtures and the pinned real-wrangler script) run in CI weekly, and the production walk has recorded the Wasabi streamed-PUT result.

## Source + Goal linkage

- **Source:** [ADR112](../../../../docs/ADR112-object-storage.md), filed by `/pmx adr 112 to w21` on 2026-11-15; ADR112 tasks t003, t005, t007 and the Contract B parts of t008 and t009, reorganized into three milestones.
- **Goal linkage:** ADR008 Render parity first, and the user's 2026-11-14 goal: serve customers' object needs while Render's early-access contract has no bucket, CORS, lifecycle or public access, by also serving the contract of `wrangler r2`.
- **Expected outcome:** A customer manages buckets, CORS and lifecycle with a tool they already have, and the platform meters and cleans up what they store.
- **Why now:** Needs the registry and client from w21/m415; the sweep and purge are what keep the store from accruing cost for deleted workspaces.
- **Render parity task:** included. The Cloudflare-shaped REST surface is customer-facing; the reference product is Cloudflare R2 and wrangler (Render has none), and the shared-data check keeps the Render contract true.

## Open decisions

- Recorded recommendation, not a user decision: every bex-chosen value (caps, error-code subset, key rule, regions) is unsigned in ADR018 until the user signs it.
- Recorded recommendation: if Wasabi rejects a streamed presigned PUT, fall back to a single `PutObject` with the known length rather than spooling.
- Not filed: ADR112 t010 (pricing) and the S3 API with tenant credentials (D10); both wait for the user.

## Unblock (2026-11-15)

- **Source:** the user's instruction in session on 2026-11-15 (the gate is not a missing deploy; deploy it yourself) and the verified evidence below. This is not a decision on any ADR018 divergence: those stay unsigned.
- **Gate cleared (verified 2026-11-15):** the production cluster is reachable from this checkout. `HCLOUD_TOKEN` is in `.env` and the SSH key is `~/.ssh/bex` (the default of `scripts/fetch-app-kubeconfig.sh`, and `BEX_SSH_PRIVATE_KEY_FILE` in `.env.example`). The earlier notes named `~/.ssh/id_bex`, a stale name copied from older runbooks; that key was never needed. Recipe: `HCLOUD_TOKEN="$(grep '^HCLOUD_TOKEN=' .env | cut -d= -f2-)" bash scripts/fetch-app-kubeconfig.sh <path outside the repo>`, then `KUBECONFIG=<path> kubectl ...`. Never print the token or the kubeconfig, and delete the file when the work is done.
- **Done on production 2026-11-15, by the orchestrator:** `provision` (kept both buckets and both key pairs, minted `BEX_OBJECT_STORAGE_GATEWAY_TOKEN`), `verify` (every check passes, including the shared-account denials), `install` (`bex-system/bex-object-storage` and `bex-system/bex-object-storage-read`, both carrying the same gateway token), then a rolling restart of `bex-api` (2/2) and `bex-objectgw` (2/2), both Ready. Argo already runs the code of w21/m417 (bex-api has the three `BEX_OBJECT_STORAGE_*` names in its environment). No round trip has been walked yet.
- **Remaining:** the walks and each closeout (below). Do not run `provision` or `install` again: both Secrets exist. The walks may create test buckets, objects and keys, and must delete them afterwards. For a token, sign in as the QA user with `bash scripts/qa-login.sh` (`.claude/skills/qa-find-bugs/SKILL.md`; never type the password). Whether the QA user can reach a second workspace is not verified: if a step needs one and there is none, park that task naming exactly that.
- **Reference-product basis:** unchanged: Render's `ea objects` for Contract A, Cloudflare R2 and wrangler for Contract B and public reads.
- **This milestone's next steps:** t013 (real wrangler against production, 300 MiB through Traefik, Render and wrangler on the same data) after w21/m415 t010, then t017 Closeout.

## Closeout (2026-11-15)

The production walk of Contract B passed (t013) and found one real defect, fixed in the same run: Cloudflare's proxy in front of `api.bex.co` refuses a request body over 100 MiB, so the Cloudflare-shaped routes are also served on the DNS-only `objects.onbex.co` (6316803e1). Not run live, with the reason recorded in t013 and t017: a workspace delete's purge (a second workspace needs a payment method), an `Age` lifecycle expiry, and the 10,000-deletion and quota caps. [w4/678](../../../w4/678.md) (a deleted `render-<region>` system bucket is recreated under a new id) is filed separately.

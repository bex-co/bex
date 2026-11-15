# w21 · m417 — Public buckets: `dev-url` and objectgw serve a bucket read-only at `pub-<obk-id>.onbex.co`, with caps, an egress alert and a kill switch

**Worker:** worker21 **Goal:** a workspace admin publishes a bucket with `wrangler r2 bucket dev-url enable` and anyone can read its objects over the web without a credential, bounded so it cannot take down the shared Wasabi account. **Status:** todo (t001–t007 and t009–t011 done; t008 and t012 remain)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | `dev-url` routes, the `pub-` slug reservation and the host lookup route for objectgw — **DONE** | 50m | w21/m416/t001, w21/m416/t002 |
| [t002](done/t002.md) | objectgw core: host lookup, GET and HEAD with `Range` and conditional requests, 404 for everything else — **DONE** | 60m | t001, w21/m415/t003 |
| [t003](done/t003.md) | objectgw headers: `nosniff`, CORS with preflight from the bucket's rules, a bounded cache and byte counting — **DONE** | 50m | t002, w21/m416/t005 |
| [t004](done/t004.md) | Rate and bandwidth caps and the month-to-date public egress alert — **DONE** | 50m | t003 |
| [t005](done/t005.md) | Deploy objectgw: image, manifests, the read-only Secret, and the kill-switch runbook — **DONE** | 55m | t002 |
| [t006](done/t006.md) | Route `pub-*` hosts to objectgw through Traefik and check the wildcard certificate covers them — **DONE** | 45m | t005 |
| [t007](done/t007.md) | Public read tests: private, unknown, `Range`, CORS, listing and keys — **DONE** | 45m | t003, t004, t006 |
| [t008](t008.md) | Production walk for public reads: enable `dev-url`, read with `Range` and CORS, disable, and measure the kill switch | 50m | t006, t007, w21/m416/t013 |
| [t009](done/t009.md) | Render parity (Cloudflare R2 reference; Render has no public buckets) — **DONE** | 30m | t007 |
| [t010](done/t010.md) | Simplify — **DONE** | 20m | t009 |
| [t011](done/t011.md) | Test coverage — **DONE** | 45m | t009, t010 |
| [t012](t012.md) | Closeout | 10m | t008, t011 |

## Definition of done

1. `dev-url enable` on a bucket makes its objects readable at `pub-<obk-xid>.<BEX_BASE_DOMAIN>` with `Range`, conditional requests and the bucket's CORS rules; `disable` stops it; private is the default.
2. The gateway cannot list, write or read a private or deleted bucket, and a service cannot take a `pub-` host.
3. Per-bucket and per-workspace rate and bandwidth caps and an egress alert (with its panel) exist, and scaling objectgw to zero is a documented and measured kill switch.
4. Production shows a public object read with `Range` and CORS, a disable that takes effect, and the measured kill-switch answer.

## Source + Goal linkage

- **Source:** [ADR112](../../../../docs/ADR112-object-storage.md), filed by `/pmx adr 112 to w21` on 2026-11-15; ADR112 tasks t006 and t009 (public part), reorganized into three milestones.
- **Goal linkage:** ADR008 Render parity first, and the user's 2026-11-14 goal: serve what customers need from objects; Render has no public access, Cloudflare R2 does, so bex follows R2's `dev-url`.
- **Expected outcome:** A customer hosts public assets (images, downloads) from a bucket without running a server, and the platform can switch it off.
- **Why now:** Public reads are the case most likely to break Wasabi's egress fair use on the shared account, so the caps, alert and kill switch land with the feature, after the store and wrangler contract exist.
- **Render parity task:** included. `dev-url` is a customer-facing REST surface and the `pub-` host a customer-facing behavior; the reference is Cloudflare R2 (Render has none).

## Parked (2026-11-15)

Everything that can be built, tested and checked offline is done: `dev-url`, objectgw, its caps and counting, the egress gauge and alerts, the manifests, the routes, the kill-switch runbook, the docs and the parity comparison (t009 no longer waits for t008, as in m415 and m416, because it needs no production). What remains needs the production cluster: **t008** and so **t012**. Before the walk, `scripts/object-storage-s3-credentials.sh provision` must run once more (it keeps the Wasabi pairs and mints `BEX_OBJECT_STORAGE_GATEWAY_TOKEN`) and `install` must write both Secrets. The walk also resets the first-value caps and the two alert thresholds and replaces the expected Traefik answer with a measured one. Needs: the user's read access to the production cluster (`/unblock w21`).

## Open decisions

- User-changeable, recommendation recorded: the public host lives on `onbex.co` (ADR112 D4); it shares a registrable domain with tenant apps, the same risk class as static sites.
- Recommendation, not a decision: the kill switch is scaling objectgw to zero; an exact-404 switch would need a runtime flag store the backend does not have.
- Not filed: custom domains, a CDN and signed public URLs (ADR112 D4 and D10).

## Unblock (2026-11-15)

- **Source:** the user's instruction in session on 2026-11-15 (the gate is not a missing deploy; deploy it yourself) and the verified evidence below. This is not a decision on any ADR018 divergence: those stay unsigned.
- **Gate cleared (verified 2026-11-15):** the production cluster is reachable from this checkout. `HCLOUD_TOKEN` is in `.env` and the SSH key is `~/.ssh/bex` (the default of `scripts/fetch-app-kubeconfig.sh`, and `BEX_SSH_PRIVATE_KEY_FILE` in `.env.example`). The earlier notes named `~/.ssh/id_bex`, a stale name copied from older runbooks; that key was never needed. Recipe: `HCLOUD_TOKEN="$(grep '^HCLOUD_TOKEN=' .env | cut -d= -f2-)" bash scripts/fetch-app-kubeconfig.sh <path outside the repo>`, then `KUBECONFIG=<path> kubectl ...`. Never print the token or the kubeconfig, and delete the file when the work is done.
- **Done on production 2026-11-15, by the orchestrator:** `provision` (kept both buckets and both key pairs, minted `BEX_OBJECT_STORAGE_GATEWAY_TOKEN`), `verify` (every check passes, including the shared-account denials), `install` (`bex-system/bex-object-storage` and `bex-system/bex-object-storage-read`, both carrying the same gateway token), then a rolling restart of `bex-api` (2/2) and `bex-objectgw` (2/2), both Ready. Argo already runs the code of w21/m417 (bex-api has the three `BEX_OBJECT_STORAGE_*` names in its environment). No round trip has been walked yet.
- **Remaining:** the walks and each closeout (below). Do not run `provision` or `install` again: both Secrets exist. The walks may create test buckets, objects and keys, and must delete them afterwards. For a token, sign in as the QA user with `bash scripts/qa-login.sh` (`.claude/skills/qa-find-bugs/SKILL.md`; never type the password). Whether the QA user can reach a second workspace is not verified: if a step needs one and there is none, park that task naming exactly that.
- **Reference-product basis:** unchanged: Render's `ea objects` for Contract A, Cloudflare R2 and wrangler for Contract B and public reads.
- **This milestone's next steps:** t008 (enable `dev-url`, read with `Range` and CORS, disable, measure Traefik at zero replicas, reset the first-value caps and the two alert thresholds from measurement) after w21/m416 t013, then t012 Closeout. The Secrets and the gateway token the gate named are in place. For the kill-switch measurement scale `bex-objectgw` to zero for as short a time as the measurement needs, then restore 2 replicas and confirm Ready; no public bucket has customers yet.

## QA note — 2026-11-15: a service whose slug is already `pub-<xid>` goes live and is unreachable, because objectgw's route outranks it

Recorded by `/qa-find-bugs-cli` sweep 78 (nonce 03b0) on production pin `b9f42ac4648e`, the first pin that carries `a378042b9`. Released `bex` v0.6.0, QA human device login, `bex-canary`. Not a status change. Both fixtures are deleted (`GET` 404).

```text
# 20:22:46 UTC, 82 s after the pin write-back (20:21:24), before bex-api had rolled
$ bex services create --name pub-dbt15qg9bdf60shrsoog --type web_service --image docker.io/traefik/whoami:v1.10.3 --plan free -o json
  → srv-dbt195nfctss7387gfkg, slug pub-dbt15qg9bdf60shrsoog (no suffix), url https://pub-dbt15qg9bdf60shrsoog.onbex.co
  deploys list: update_in_progress → live (20:25:57)
  curl https://pub-dbt15qg9bdf60shrsoog.onbex.co/ → 503 "public buckets are not available" (objectgw), before and after live

# 20:26:25 UTC, after the rollout: the reservation works
$ bex services create --name pub-dbt1asc9mhspr7e1c2g0 … -o json
  → srv-dbt1as80p4tc73b3d630, slug pub-dbt1asc9mhspr7e1c2g0-zhnp; live; the -zhnp host answers 200 (the app)
  curl https://pub-dbt1asc9mhspr7e1c2g0.onbex.co/ → 503 from objectgw, as designed
```

- **Cause:** `lego/operator/config/objectgw/ingressroute.yaml:43,66` gives both `pub-` routes `priority: 10000`, above every App route. The comment at `:22-28` relies on "Service slugs cannot take the pub- prefix … so no service host is ever shadowed by this route". The reservation (`publichost.Reserved`, applied only in `store.PGStore.CreateApp`, `lego/backend/internal/store/store.go:1117-1123`) guards new creates only. Two kinds of App are left with a bare `pub-<xid>` slug:
  - any App created before `a378042b9` reached production;
  - any App created while objectgw's route was already live but the previous bex-api still answered. That is how the first fixture above was made.
- **Effect:** the service reports `live` in the CLI, REST and the dashboard, but every request to its URL reaches objectgw and gets a 503 (404 once credentials are installed). Nothing tells the owner.
- **Blast radius (unverified):** no production query was run. The affected rows are `apps.slug ~ '^pub-[0-9a-v]{20}$'` across all workspaces; the name shape is unusual, so the count is probably 0 apart from this sweep's deleted fixture. The rollout window happens once.
- **Fix options:**
  - **Recommended:** lower the two routes' `priority` below App routes but above the priority-1 fallbacks (for example `2`). An exact service host, whose rule-length priority is a few dozen, then wins, while unknown `pub-` hosts still reach objectgw ahead of the activator fallback. The reservation keeps new services off bucket hosts. Bucket xids are time-ordered, so an old service slug cannot match a bucket minted later.
  - **Or:** a one-off migration that re-suffixes existing `pub-<xid>` slugs. That changes those services' URLs, which already do not serve.
  - Either way, correct the comment at `ingressroute.yaml:26-28`, and have t008's production walk count matching slugs.
- **Acceptance:** a service whose slug is a bare `pub-<xid>` serves its own app on its URL, `https://pub-<unknown xid>.onbex.co/` still reaches objectgw, and a new `pub-<xid>` service still gets a suffix.
- Minor (a rare name, plus a one-time rollout window), but a silent outage for any service it hits. ~30m.

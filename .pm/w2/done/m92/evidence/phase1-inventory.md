# w2/m92 · Phase 1 inventory (read-only)

**Date:** 2026-09-08 · **Cluster:** `hetzner-prod` · **Mode:** read-only (no `--apply`)  
**Instruments:** `kubectl` App/Secret/Deployment inventory + `go run ./cmd/registry-migrate --all` dry-run via local port-forward to in-cluster Zot (`127.0.0.1:15000` → `bex-registry/zot:5000`). Builder password loaded from `bex-build/bex-registry-push` for the dry-run only (never printed or recorded).

## Cluster snapshot

| App (ns/name) | WS label | Tombstone | `status.image` shape | Deploy image shape | Phase |
| --- | --- | --- | --- | --- | --- |
| `default/hello-static` | _(none)_ | — | unlabeled demo | — | Running — **out of scope** (unlabeled) |
| `tea-d98210…/…-agentmarketcap-1` | labeled | no | **legacy** `…/A:gen-230@…` | **legacy** | Running |
| `tea-d98210…/…-beancount-cms-v2` | labeled | no | scoped `…/W/A:gen-460@…` | scoped | Deploying |
| `tea-d98210…/…-beancount-forum` | labeled | no | scoped | scoped | Running |
| `tea-d98210…/…-block-eden-mono` | labeled | no | scoped | scoped | Running |
| `tea-d98210…/…-eden-cms-v2` | labeled | no | scoped | scoped | Running |
| `tea-da1eg9…/…-market-size` | labeled | no | _(empty)_ | _(no Deploy)_ | Failed |
| `tea-da2isi…/…-tianpan-v4-web` | labeled | no | scoped | scoped | Running |

Same-name collisions across namespaces: **none**.

`BEX_REGISTRY_DUAL_READ` on `bex-controller-manager`: **unset (off)**. Must be enabled for the supervised migration window before Phase 2 (runbook / operator CLAUDE.md) — record as a Phase-2 precondition, not a Phase-1 mutation.

## Migration worklist (labeled, not tombstoned)

Dry-run plan matches the kubectl before-picture: every labeled App plans `legacy A → W/A` with `tombstone: yes (no blob delete)`. Tag copy counts (legacy tags whose dest digest is empty):

| App | Legacy tags to copy | Notes |
| --- | ---: | --- |
| `…-agentmarketcap-1` | 5 (`gen-208`…`gen-211`, `gen-230`) | **Live Deploy still on legacy `gen-230`** — Phase 3 redeploy required |
| `…-beancount-cms-v2` | 5 (`gen-224`…`gen-228`) | Live already on scoped `gen-460` (never on legacy); copy is historical tags only |
| `…-beancount-forum` | 4 | Live already scoped |
| `…-block-eden-mono` | 4 | Live already scoped |
| `…-eden-cms-v2` | 5 | Live already scoped |
| `…-market-size` | 0 | Failed App, no image; tombstone-only plan |
| `…-tianpan-v4-web` | 0 | Live already scoped; tombstone-only (legacy repo empty of listed tags) |
| **Total** | **23 tag copies** | Plus 7 tombstone stamps; **0 blob deletes** |

Static prefixes: no labeled App has `status.staticPrefix` set; none of the labeled Apps are `static_site`. S3 prefix copy expected empty/no-op for this worklist (bucket `bex-static` / Wasabi `eu-central-2` per operator env — credentials not used in Phase 1).

Pull Secrets / htpasswd: both legacy (`reg-pull-A` / `app-A`) and scoped (`reg-pull-W-A` / `app-W-A`) rows exist for the active Apps — consistent with dual-identity minting without tombstones yet. Orphaned credentials for deleted Apps (`blockeden-forum`, `tianpan-forum`, `static-site`, …) noted for optional later cleanup; **out of scope** for this migration (no live App).

## Maintenance tolerance (Phase 3 ordering)

| App | Tolerance / notes | Suggested Phase-3 order |
| --- | --- | --- |
| `…-market-size` | Already Failed; no Deploy — tombstone-only, lowest risk | 1 (first) |
| `…-tianpan-v4-web` | Running, already scoped image — redeploy is confirm-only | 2 |
| `…-beancount-forum` | Running, already scoped | 3 |
| `…-block-eden-mono` | Running, already scoped | 4 |
| `…-eden-cms-v2` | Running, already scoped | 5 |
| `…-beancount-cms-v2` | **Deploying** at inventory time — wait for healthy before Phase 3 | 6 (after Ready) |
| `…-agentmarketcap-1` | Running **on legacy image** — only App that must cut over image ref; schedule a short window | 7 (last, explicit window) |

## Mutations

**None.** Inventory + dry-run only. No `--apply`.

## Cross-check

`registry-migrate` dry-run worklist ≡ labeled non-tombstoned Apps from `kubectl get apps` (7 Apps). Unlabeled `hello-static` excluded by design.

## Refresh (read-only, 2026-09-08 later)

`kubectl` re-check on `hetzner-prod` confirms the same 7 labeled Apps, **zero** tombstones, and the same shapes: only `…-agentmarketcap-1` still on a **legacy** image ref; `…-beancount-cms-v2` still **Deploying** on scoped `gen-460`; `BEX_REGISTRY_DUAL_READ` still unset on `bex-controller-manager`. Worklist unchanged pending STOP authorization.

# w2/m92 · Phase 2 apply (tenant worklist)

**Date:** 2026-09-08 · **Cluster:** `hetzner-prod`  
**Auth:** recorded in `t003.md` (`/goal implement .pm/w2/m92/README.md` change-window directive)

## Preconditions

- `BEX_REGISTRY_DUAL_READ=1` set on `bex-controller-manager`; rollout succeeded
- Zot port-forward `127.0.0.1:15000` → `bex-registry/zot:5000`
- `skopeo login` to `127.0.0.1:15000` as `bex-builder` (required for copy; first `--apply` attempt failed with `authentication required` before login)
- Pre-apply dry-run matched t001: **23** tag copies + 7 tombstones, **0** blob deletes

## Execution

| Namespace | Result |
| --- | --- |
| `tea-da1eg9…` (`market-size`) | migrated (tombstone-only) |
| `tea-da2isi…` (`tianpan-v4-web`) | migrated (tombstone-only) |
| `tea-d98210…` (5 Apps) | first attempt aborted on skopeo auth mid-`agentmarketcap`; **retry after login succeeded** — all five `migrated` |

Log: `.pm/w2/m92/evidence/phase2-apply.log` (contains registry hostnames/tags only; no passwords).

## Verify

| Check | Result |
| --- | --- |
| All 7 labeled Apps `identity-tombstone=true` | yes (unlabeled `hello-static` unchanged) |
| Spot digest `agentmarketcap:gen-230` legacy == `W/A` | `DIGEST_OK` `sha256:07d198c2…` |
| Spot digest `beancount-forum:gen-64` | `DIGEST_OK` `sha256:ba0a4c64…` |
| Legacy tags retained (+ `bex-tombstone`) | yes on `agentmarketcap` |
| Blob deletes | **none** |
| Serving regression / ImagePullBackOff burst | none attributed to migrate (pre-existing cms-v2 Pending; unrelated build pods Init) |

## Notes for Phase 3

- Only `…-agentmarketcap-1` still has **legacy** `status.image` / Deploy ref shape — must cut over
- Others already on scoped images; confirm-only rolls
- `…-beancount-cms-v2` still Deploying/Pending (capacity) — skip forced restart

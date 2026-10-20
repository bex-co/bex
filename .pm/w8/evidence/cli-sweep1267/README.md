# CLI sweep 1267 — 100 real Render-exported Blueprints through `bex blueprints validate` (live, 2026-10-20)

Read-only corpus sweep plus one create probe. Production `https://api.bex.co/v1/`, pin `9cace6b6e25d`, bex-canary, HEAD-built pinned CLI, human device-login grant.

## Corpus

GitHub code search `Exported from Render` (filename `render.yaml`) gave 100 distinct files, each starting with Render's export header `# Exported from Render on <date>`. Each file ran once through `bex blueprints validate <file> -o json`, about 2 s apart, with no fixtures.

| Outcome | Files |
| --- | --- |
| `valid: true` | 4 |
| any `per-resource region placement is not available on bex` | 89 |
| **only** region errors | 75 |
| previews / `pullRequestPreviewsEnabled` (documented non-goal) | 9 |
| service name not a DNS label (`Visual_Sort`, `Results`, `Mobilerotaryai`, `raminxz.github.io`; deliberate per ADR020 and ADR049:356) | 4 |
| author-broken files: invalid YAML (tabs, stray lines; PyYAML agrees), `autoDeployTrigger: "on"`, `env: java`, `region: Vietnam`, boolean env value, `sizeGB`/`version` keys | 9 |

Regions in the corpus: `oregon` 63, `virginia` 33, `singapore` 27, `frankfurt` 19.

## Create probe

`bex services create --name qa-20261020-<nonce>-region --type web_service --plan free --image docker.io/ealen/echo-server:0.9.2 --region oregon -o json --confirm` → exit 0, `serviceDetails.region: "fsn1"`, empty stderr. `srv-dbbvf71v5vjs73b9h9gg` was deleted (204, then GET 404).

Filed as [w8/152](../../152.md): create normalizes the region while Blueprints refuse it.

## Also observed

- Deploy run 39504771495 (`bbf1285f7`, the buildkit pre-pull retry fix) failed `test-operator`: `TestValkeyPersistenceEngine/7/all_mode_pairs` hit the package's 30 m `go test` timeout on the arm64 runner. This is a CI capacity issue, not a product finding. The w8 fixes are still not deployed.

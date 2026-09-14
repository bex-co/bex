# Blueprint validation error attribution

Captured 2026-09-14 against bex's pinned Render schema (`lego/backend/internal/apps/schema/render.yaml.json`, SHA `RenderBlueprintSchemaSHA256`) and `CompileBlueprintSource` / `ValidateBlueprint`.

## What bex reports after w4/m102

| Manifest | Result |
| --- | --- |
| `examples/static-site/render.yaml` (no `plan`) | `valid: true`, non-null plan |
| Same shape plus `plan: free` | `additional properties 'plan' not allowed` at `/services/0/plan` |
| Static site with no publish dir | `staticPublishPath is required for a static_site` |
| `type: nonsense` | `value must be one of 'web', 'worker', 'pserv', 'cron', 'keyvalue', 'redis'` |
| Env-var form fault on an otherwise valid web service | names the env-var field, not `redisServer` / `ipAllowList` |

REST `POST /v1/services` for a static site still says `publishPath is required for a static_site`. GraphQL, MCP, and REST blueprint validate share one problem list.

## Render.com live validator

Not captured this run (no Render account session). Render's schema is the pin above: `staticService` lists `staticPublishPath` and has no `plan`. bex does not change that document. If a later live capture of Render's validate endpoint disagrees on message wording, treat wording as independent of acceptance.

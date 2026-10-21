# CLI sweep 1276 — every official `render-examples` Blueprint through `bex blueprints validate` (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `9cace6b6e25d`, HEAD-built pinned CLI. Read-only; no fixtures.

The `render-examples` org has 198 non-archived repos. 150 have a root `render.yaml`. Of those, 76 pass Render's pinned Blueprint schema locally (jsonschema 4.26), and 39 pass `bex blueprints validate`.

Results for the 52 that pass Render's schema but fail on bex:

| Blocking categories | Count | Disposition |
| --- | --- | --- |
| region only | 16 | w8/152 (decision) |
| `PORT` env var only | 12 | deliberate (ADR018:137, w2/m95). Render's own examples set `PORT` |
| workspace Postgres quota | 12 | environmental (the QA workspace holds its free database) |
| `fromService.envVarKey: RENDER_EXTERNAL_URL` | 3 (+1 file failing Render's schema too) | w8/155 addendum: `ai-hedge-fund-on-render`, `answer-arena`, `strapiconf2022-workshop-strapi`, `RAG-chatbot-template` |
| previews (± region) | 5 | non-goal |
| name not a DNS label | 1 | deliberate |
| other | 3 | 2× disk `sizeGB` below Hetzner's 10 GB minimum (recorded); 1× **w8/160** (`plan: pro-4gb` refused as "not a supported Render Postgres plan") |

Also found:

- **w8/161:** `render-examples/AEO-analytics` copies two `sync: false` variables between services. A minimal two-service reproduction (`syncref.yaml`, flat and nested under `projects[]`) is refused with `… which has no such plainly-defined variable`, against `resolveRef`'s own comment that `sync: false` targets are allowed.
- Not filed, open question: `AEO-analytics`'s build command runs `cd .. && pnpm install` from `rootDir: web`. Render's monorepo guide says "Files outside your service's root directory are not available … at build time", and Render's monorepo article says the full repo is cloned. Render's own example therefore contradicts its doc. Bex copies only `rootDir` into the native build context (sweep 1271), so this example would fail on bex. There is no live Render oracle to settle which reading is right, so it is recorded here only.

# blueprint-refs — do Blueprint references reach a running process?

The fixture for [w8/375](../../.pm/w8/blocked/375.md): a free-only [`render.yaml`](render.yaml) whose web service takes every kind of Blueprint reference a free service can, and reports at `GET /` what each one resolved to. `blueprints validate` proves a reference parses; this shows its value arriving.

| Reference | Declared on `blueprint-refs-app` | Expected line in the report |
| --- | --- | --- |
| `envVarGroups` + `fromGroup` | `fromGroup: blueprint-refs-group` | `GROUP_MARKER=group-marker-ok` |
| `fromService` `envVarKey` | `PEER_MARKER` from `blueprint-refs-peer` | `PEER_MARKER=peer-marker-ok` |
| `fromService` `host` of a static site | `SITE_HOST` from `blueprint-refs-site` | `SITE_HOST=<the site's hostname>` |
| `generateValue` | `GENERATED_SECRET` | `GENERATED_SECRET set length=44 base64=yes bytes=32` |
| `sync: false` | `MANUAL_SECRET` | `MANUAL_SECRET set length=0 …` when left blank |

- The two secrets are reported by length and shape only, never by value: Render documents `generateValue` as "a base64-encoded 256-bit value", which is 44 characters.
- Both web services build [`app/`](app) (`rootDir`), a busybox image whose [`report.sh`](app/report.sh) writes the report at start and serves it. Their `buildFilter` includes `examples/blueprint-refs/app/**` and ignores Markdown in it, so a push elsewhere in the repository does not redeploy them. Build filter paths are relative to the repository root, not to `rootDir`.
- Every web service is on plan `free`, and a static site takes no plan. Workers, private services and cron jobs are paid, and a `fromService` `host`, `port` or `hostport` naming a free web service is refused by design (w8/m130). So the web-to-web link is an `envVarKey` copy, and the `host` comes from the static site.

## Run it

```sh
bex blueprints validate examples/blueprint-refs/render.yaml
bex ea blueprints create --repo https://github.com/bex-co/bex \
  --path examples/blueprint-refs/render.yaml --auto-sync=false
bex ea blueprints approve <exs-id> --confirm
```

Once `blueprint-refs-app` is live, `curl` its URL and compare the report with the table above. `bex ea blueprints plan <exs-id>` should then read `no_changes`. Afterwards delete the three services, the env group and the Blueprint by ID.

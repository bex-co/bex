# CLI sweep 1240 — Render's full example Blueprint through `bex blueprints validate` (live, 2026-10-20)

Validation only; nothing was created. The bare-IP `services create` probe was refused by the pinned CLI before any request. Source: the example `render.yaml` in [Render's Blueprint reference](https://render.com/docs/blueprint-spec), retrieved 2026-10-20.

- The verbatim example → exit 1 with five errors: `databases[3].plan` 2c-8g (ADR049: only exact-size Postgres aliases), root and static-site `previews` (DO_NOT_DO), `services[0].region` (ADR049: single placement), and the `type: workflow` service (non-goal). All are documented refusals.
- After removing those, six errors remain:
  - three `ipAllowList: "198.51.100.1" is not a valid CIDR` (web service, static site, database). **Finding → [w8/142](../../142.md).**
  - the pserv `PORT` env var (w2/m95, reserved by design).
  - two database names with spaces (`private database`, `highly available database`), which ADR020/ADR009 refuse by design (DNS-label names).
- Legacy aliases `env: docker` and `type: redis` validate (`valid: true`).
- A `projects[].environments[].ipAllowList` is refused by Render's own schema (`additional properties … not allowed`), which is correct.

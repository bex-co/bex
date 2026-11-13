# w4 · m363 — A service's or environment group's secret files hold Render's 1 MB combined, not 512 KiB

**Worker:** worker4 **Goal:** a service or environment group accepts secret files up to Render's documented combined size ("The combined size of all secret files uploaded to any given service or environment group cannot exceed 1 MB"). The cap counts file contents, as Render's sentence and Kubernetes' Secret limit both do. The dashboard, REST, GraphQL and MCP all enforce that one bound, and a full-size set mounts at `/etc/secrets`. The environment-variable caps are unchanged. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Secret-file maps hold 1 MiB of contents on every writer; env-var maps keep 512 KiB | 40m | — |
| [t002](t002.md) | Prove a full-size files map materializes, projects and mounts, and fits every request-body limit on the way | 35m | t001 |
| [t003](t003.md) | The dashboard's secret-file total follows the server: contents only, 1 MiB, en and zh copy | 30m | t001 |
| [t004](t004.md) | Amend ADR018's Secret files row and ADR066 #6: the divergence and its base64 rationale go | 15m | t001, t002 |
| [t005](t005.md) | Render parity | 20m | t002, t003, t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005 |
| [t008](t008.md) | Closeout (live replay on production) | 20m | t006, t007 |

## Definition of done

All on production, in workspace `bex-canary`. The fixture is a Free web service created from New → Web Service → Existing Image: `docker.io/library/python:3.12-alpine`, Docker Command `sh -c "mkdir -p /tmp/w; cp -r /etc/secrets /tmp/w/s 2>/dev/null; cd /tmp/w && exec python -m http.server $PORT"`. That serves `/etc/secrets/<name>` at `https://<name>.onbex.co/s/<name>`.

1. **One large file is accepted.** `PUT /v1/services/<id>/secret-files/qa-big1.txt` with a 614,400-byte `content` answers 2xx. run sf13 answered `400 {"error":"bad request: total secret file size limit of 524288 bytes exceeded"}` for this exact request, with only a 46-byte `app.conf` already stored.
2. **The bound is Render's, on contents.** Files whose contents total exactly the chosen bound (t001 records it: at least 1,000,000 bytes, at most 1 MiB) are accepted. One more byte is refused with a 400 that names the limit. GraphQL `patchServiceEnvironment` and MCP `patch_service_environment` agree byte for byte. A 600 KiB environment-variable set is still refused (that cap is not this milestone's).
3. **The dashboard admits it and it mounts.** Environment → Edit → Upload files → a 614,400-byte `cert600k.txt` adds a row with no inline limit alert, and Save and deploy is enabled. run sf13 marked the row `[invalid]` with "Secret files are limited to 500 and 512 KiB in total per service or group." and disabled Save. After Save and deploy the deploy goes Live, and `curl -s https://<name>.onbex.co/s/cert600k.txt | wc -c` prints `614400`.
4. **An environment group does the same.** A group's 614,400-byte secret file saves, and on a linked service it mounts at `/etc/secrets/<file>` with the same byte count.
5. **The ledger says so.** `docs/ADR018-render-parity.md`'s Secret files row no longer records a 512 KiB divergence. ADR066 #6 no longer says the projection is base64-encoded into the Secret's 1 MiB.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, run sf13, 2026-11-13 06:01–06:13 UTC (`muse.env`, workspace `bex-canary`; pin `4c3c68e6b05b`). The probes are under § Finding. Screenshot `.playwright-mcp/qa-env-1.png` (local, gitignored) shows the 600 KiB upload marked invalid. **Severity:** minor, parity. Secret-file sets over 512 KiB are uncommon, but when one exists, a configuration Render accepts is refused outright.
- **Governing docs:**
  - The cap and its rationale are in [ADR066](../../../docs/ADR066-security-review-round11.md) #6 and in [ADR018](../../../docs/ADR018-render-parity.md)'s Secret files row (w1/m147, 2026-09-15). That row records the 512 KiB cap as a divergence from Render's 1 MB.
  - Secret files themselves are in [ADR013](../../../docs/ADR013-secrets.md).
  - [AGENTS.md](../../../AGENTS.md) § Rules (user decision 2026-10-17, restated 2026-10-20) says "A divergence stands only when ADR018 records it with the user's sign-off". The row predates that rule and carries no sign-off. w1/m147 t005 chose the cap in the same session that recorded it ("Record it as a divergence; do not raise the server cap to match").
- **Goal linkage:** Render parity (ADR008 § Render parity first) on journey 4, environment variables and secret files. A Render customer whose service mounts a CA bundle, a keystore exported as base64, or several certificates totalling more than 512 KiB cannot move that configuration to bex.
- **Expected outcome:** any secret-file set Render accepts is accepted, stored and mounted by bex, on every surface.
- **Why now:**
  - Parity rule: an unsigned divergence must be amended, not kept.
  - The recorded reason does not hold. `k8s.io/api@v0.37.1` `core/v1/types.go:8288` says "The total bytes of the values in the Data field must be less than MaxSecretSize bytes", and `MaxSecretSize = 1 * 1024 * 1024` (`:8326`). `Data` is `map[string][]byte` (raw bytes; base64 is only the JSON wire form), and keys are not counted. So 1 MB of contents fits one `<name>-files` Secret. No split across Secrets is needed, and the projected volume already merges the service's Secret with every linked group's (`lego/operator/internal/controller/deployment_projection.go:671-677`).
- **Render parity task included (t005):** the change moves a limit on REST, GraphQL, MCP and the dashboard editor.

## Finding (run sf13)

**Fixture:** Free web service `qa-20261113-sf` (`srv-dbragho2er5s73bkielg`, owner `tea-daif693dqjvc73e7as3g`). It was created from the dashboard with the image and Docker Command above, plus one secret file, `app.conf`. Deleted at 06:13 (`DELETE` 204, then `GET` 404).

- **REST, from the signed-in page** (`fetch(…, {credentials:'include'})`):

  ```text
  PUT /v1/services/srv-dbragho2er5s73bkielg/secret-files/qa-big1.txt  {"content": "b" × 614400}
  → 400 {"error":"bad request: total secret file size limit of 524288 bytes exceeded","id":"bad_request","message":"bad request: total secret file size limit of 524288 bytes exceeded"}
  ```

  The same request for `qa-big2.txt` returned the same 400. Only `app.conf` (46 bytes) was stored.

- **Dashboard:** Environment → Edit → Upload files → `cert600k.txt` (614,400 bytes). A row was added, its name field marked `[invalid]`, with the alert "Secret files are limited to 500 and 512 KiB in total per service or group." Save and deploy and Environment save options were disabled. The draft was cancelled.
  - The upload itself admits files up to `MAX_SECRET_FILE_BYTES = 1024 * 1024` (`dashboard/src/features/services/lib/environment-draft.ts:14`), and its skip message says "files no larger than 1 MiB". The total check then refuses a single 600 KiB file at `MAX_ENVIRONMENT_MAP_BYTES = 512 * 1024` (`:24`, `:272`).
- **Control (passes):** below the cap, propagation is exact. `app.conf`'s multi-line contents (indentation, blank line, final LF, then a Unicode and tab replacement with no final LF) and seven imported variables reached the running process after each Save and deploy. The deploy list and Events agreed (`service_environment_changed`, then `deploy_started` with `envUpdated: true` and `deploy_ended` `succeeded`).
- **The code:**
  - `lego/backend/internal/secrets/service.go:969-973`: `maxSecretMapBytes = 512 << 10`, shared by `envMapWithinQuota` (`:977-985`) and `filesMapWithinQuota` (`:988-996`). `mapBytes` (`:1020-1026`) counts `len(k) + len(v)`.
  - The comment at `:962-968` says "The byte quota stays under Kubernetes' 1 MiB Secret ceiling". ADR018 gives the reason as "keeps each map, base64-encoded, inside the 1 MiB Kubernetes Secret". The Kubernetes contract is above (Why now).
- **Render:** [configure-environment-variables](https://render.com/docs/configure-environment-variables), re-read 2026-11-13: "The combined size of all secret files uploaded to any given service or environment group cannot exceed 1 MB."

## Unverified

- **The apiserver's own check.** `k8s.io/kubernetes`'s `ValidateSecret` is not in the module cache, so whether it refuses at `> MaxSecretSize` or `>= MaxSecretSize` was not read. t002 settles the exact bound against envtest's real kube-apiserver and etcd.
- **etcd and body limits.** Production etcd's `--max-request-bytes` (default 1.5 MiB) was not read. bex-api's REST body limit sits between 1 MiB and 4 MiB (w4/422: 1024 KiB → 400, 4096 KiB → 413). GraphQL's and MCP's limits were not probed. A 1 MiB file whose JSON escaping inflates it, through the batch `PATCH …/environment` with other changes, may hit a body limit first. t002 measures each one.
- **Environment groups were not probed live this round.** The shared check is `secrets.ValidateFilesMapQuota` (`lego/backend/internal/envgroups/service.go:612`).
- **Render's exact number.** "1 MB" may mean 1,000,000 bytes or 1 MiB. The DoD accepts either reading by requiring at least 1,000,000 bytes.

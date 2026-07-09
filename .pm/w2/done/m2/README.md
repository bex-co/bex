# w2 · m2 — Deploy-from-chat + HMAC git webhook

**Worker:** worker2 **Goal:** Make "deploy this" one agent action — a single call takes a repo + `bex.yml` to a live https URL, and a signed git push redeploys. Delivers pillar 4. **Status:** DONE (t001–t003 DONE 2026-07-08; t004 live acceptance PASSED 2026-07-09 — MCP `deploy` → in-cluster build → Running → curl 200, then signed webhook → gen bump → new build served)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Deploy verb over `Core.Create`: {repo, bex.yml} mapped onto Render's create surface (amended 2026-07-08 — no bespoke `/v1/deploy`) — **DONE** | 30m | w2/m4, w1/m5 |
| t002 | Expose `deploy` as an MCP verb — **DONE** | 20m | t001, w2/m1/t001 |
| t003 | HMAC-verified git webhook endpoint → redeploy on push — **DONE** | 30m | t001 |
| t004 | End-to-end acceptance: agent deploy → live URL; push → redeploy — **DONE** | 25m | t001,t002,t003 |

## Definition of done

One call (REST or MCP) with a repo + `bex.yml` yields a live https URL; a later git push carrying a valid HMAC signature triggers a redeploy — no dashboard, no `kubectl`. Intent is written as an `App` CR spec; the operator converges it.

## Progress (2026-07-08)

The product code for the whole loop is implemented and unit/integration-tested (`lego/backend`):

- **t001** — `apps.Service.Create` (create-or-update upsert; writes the `App` CR directly, the hand-applied path) + `Deploy` (the `bex.yml` → `CreateRequest` mapper in `deploy.go`, mirroring `scripts/app-apply.sh`). Repeating a deploy for the same name redeploys (re-applies the spec and bumps `spec.restartedAt`), never duplicates. Since w2/m4's create surface hadn't landed, this task also built the `Create` verb it was meant to ride, plus `POST /v1/services` (REST, 201), `createService` (GraphQL), and `create_web_service` (MCP).
- **t002** — MCP `deploy` (`{repo, branch?, bexYaml}`) and `create_web_service` tools over the same `Create`; return the service object to poll to Ready.
- **t003** — `POST /v1/webhooks/git` (`apps/webhook.go`): constant-time HMAC-SHA256 verify (`X-Hub-Signature-256`) against `BEX_WEBHOOK_SECRET`, mounted **outside** the OAuth gate; a valid push redeploys every App whose `spec.repo`+branch match the pushed repo, unsigned/mismatched → 401 no-op, unset secret → 503.

**t004 (live acceptance) is blocked** on **w1/m5** (in-cluster builds): a repo-backed create/deploy converges to a live URL only once builds run in-cluster. The handler-level loop (deploy → CR written → webhook → redeploy) is proven by tests today; the live `hello-go` build → `*.onbex.co` URL awaits w1/m5. `examples/hello-go/bex.yml` is in place as the acceptance target. Docs: `docs/bex-api.md` (create surface, deploy-from-chat, push-to-deploy webhook).

## Source

`docs/vision.md` pillar 4 (deploy-from-chat) + roadmap item 2 (wake activator + HMAC webhook); `docs/control-plane.md` request flow. **Cross-workstream:** needs w2/m4 (Render-shaped create surface — t001 amended 2026-07-08 to ride `Core.Create` instead of a bespoke `POST /v1/deploy`), w1/m2 (control plane) and w1/m5 (in-cluster builds) first.

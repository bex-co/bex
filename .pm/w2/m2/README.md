# w2 · m2 — Deploy-from-chat + HMAC git webhook

**Worker:** worker2 **Goal:** Make "deploy this" one agent action — a single call takes a repo + `bex.yml` to a live https URL, and a signed git push redeploys. Delivers pillar 4. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Deploy verb over `Core.Create`: {repo, bex.yml} mapped onto Render's create surface (amended 2026-07-08 — no bespoke `/v1/deploy`) | 30m | w2/m4, w1/m5 |
| t002 | Expose `deploy` as an MCP verb | 20m | t001, w2/m1/t001 |
| t003 | HMAC-verified git webhook endpoint → redeploy on push | 30m | t001 |
| t004 | End-to-end acceptance: agent deploy → live URL; push → redeploy | 25m | t001,t002,t003 |

## Definition of done

One call (REST or MCP) with a repo + `bex.yml` yields a live https URL; a later git push carrying a valid HMAC signature triggers a redeploy — no dashboard, no `kubectl`. Intent is written as an `App` CR spec; the operator converges it.

## Source

`docs/vision.md` pillar 4 (deploy-from-chat) + roadmap item 2 (wake activator + HMAC webhook); `docs/control-plane.md` request flow. **Cross-workstream:** needs w2/m4 (Render-shaped create surface — t001 amended 2026-07-08 to ride `Core.Create` instead of a bespoke `POST /v1/deploy`), w1/m2 (control plane) and w1/m5 (in-cluster builds) first.

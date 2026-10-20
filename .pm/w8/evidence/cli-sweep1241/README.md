# CLI sweep 1241 — proxy headers a web service actually receives (live, 2026-10-20)

One free `docker.io/ealen/echo-server:0.9.2` web service (`qa-20261020-6af3cd-hdr`, `srv-dbbhh8ucbnls73997dd0`) created with `bex services create`, deleted (GET 404). Client addresses are redacted.

- Correct: `X-Forwarded-For`, `X-Real-IP`, `X-Forwarded-Proto` (`https`), `X-Forwarded-Host` and `X-Forwarded-Port` are set by the edge and forged inbound values are replaced. `Host` is preserved.
- **Finding → [w8/143](../../143.md):** forged `True-Client-IP`, `CF-Connecting-IP`, `X-Client-IP`, `CF-IPCountry`, `CF-Ray` and `Forwarded` reach the container unchanged. These are the headers Render apps are told to trust because Cloudflare sets them on Render.
- Also observed: `X-Forwarded-Server: traefik-<pod>` exposes the edge pod name. It is folded into w8/143 as optional cleanup.
- Response headers carry no `rndr-id`/`x-render-origin-server`. That is cosmetic and not filed.

# Ingress body-read budget — decision needed (w8, 2026-10-17)

**Gate for m108.** The public `web`/`websecure` entrypoints run Traefik v3.7.5 at its default `respondingTimeouts.readTimeout` of 60 s. That limit covers reading a whole request, body included, so a slow one-MiB upload is cut at 60 s with a 499 ([finding](finding.md)). Render allows web-service requests up to 100 minutes. The fix the milestone proposes is a single Helm value: `ports.{web,websecure}.transport.respondingTimeouts.readTimeout: 100m`.

## Why it isn't a one-line change

In the pinned Traefik source (`pkg/server/server_entrypoint_tcp.go` at v3.7.5), the same value is also:

- the connection-level read deadline, set before Traefik has even peeked at the first byte to tell TLS from plaintext (line 275);
- the PROXY-protocol header timeout (lines 444–449);
- Go's `http.Server.ReadTimeout` with no `ReadHeaderTimeout` (line 723), so it also bounds how slowly request headers may arrive.

Traefik v3.7.5 has no separate header or handshake timeout. Raising the budget to 100 minutes therefore lets any unauthenticated client hold a connection on the shared production edge for 100 minutes by trickling bytes or sending nothing at all. That is a slowloris exposure for every tenant and platform host behind these entrypoints. Today the same connection is dropped after 60 s.

## Options

1. **100 minutes, matching Render.** Accept the slow-client exposure, mitigated only by the Hetzner load balancer's connection limits.
2. **A bounded middle value, for example 10 minutes.** Covers ordinary large uploads on slow links, keeps the exposure at a tenth, and records the remaining difference from Render's 100 minutes.
3. **Upstream first.** Authorize an upstream Traefik PR adding a separate header/handshake timeout (`readHeaderTimeout`), then set the body budget to 100 minutes with headers still capped. Needs external submission plus a Traefik bump.

**Recommendation: option 2 now, with option 3 as follow-up** if exact parity matters.

Nothing is implemented. It is a production edge configuration change with a security trade-off, so it waits for the user's choice.

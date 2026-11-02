# w8 · m145 — The gzip-only web-service edge stops answering zstd

**Worker:** worker8 **Goal:** a browser offer (`gzip, deflate, br, zstd`) to a web service gets gzip, as ADR018 records, by running a Traefik whose gzip handler honours `encodings`. **Status:** done — 2026-11-02, verified live on production ([evidence](evidence/live-20261102.md))

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Pin Traefik v3.7.17 by digest and move bex's v3.7.5 references — **DONE** | 30m | — |
| [t002](done/t002.md) | Prove bex's edge contracts on v3.7.17 against a real Ingress — **DONE** | 60m | t001 |
| [t003](done/t003.md) | Simplify — **DONE** | 10m | t002 |
| [t004](done/t004.md) | Test coverage — **DONE** | 20m | t002 |
| [t005](done/t005.md) | Closeout: deploy and live browser-offer check — **DONE** | 30m | t003, t004 |

**Total:** 5 tasks / 150m. [Finding](finding.md).

## Definition of done

- Production Traefik runs v3.7.17 (digest-pinned, chart 41.0.0 unchanged), rolled without an edge outage.
- On a fresh Free web service, `Accept-Encoding: gzip, deflate, br, zstd` → `Content-Encoding: gzip`, decoded byte-exact; `zstd`-only, `br`-only and `identity` stay identity; m83's measured rows still hold (`TestEdgeCompressionOnTheDeployedTraefik` on the pinned image).
- bex's other edge dependencies hold on v3.7.17, verified against a real Kubernetes Ingress: request-log attribution (`KubernetesIngressNamespace`/`KubernetesIngressName` on routed and IPAllowList-refused requests), the `bexClientIPHeaders` and `bexWebsocketEgress` local plugins load and act, router byte metrics, and the backend router-key algorithm (`lego/backend/internal/core/traefik.go`).
- Fixtures deleted by ID with a 404 proof.

## Source + Goal linkage

- **Source:** [w8/228](finding.md) (`/qa-find-bugs-cli` sweep 3093, 2026-11-01). Root cause: Traefik v3.7.5 selects its gzip handler, but that handler (klauspost `gzhttp` v1.18.5) negotiates zstd itself whenever offered. Traefik v3.7.9+ passes `gzhttp.EnableZstd(false)` (`pkg/middlewares/compress/compress.go`); v3.7.5–v3.7.8 do not. Reproduced on `traefik:v3.7.5` with the m83 actual-proxy gate (`gzip, deflate, br, zstd` → zstd).
- **Goal linkage:** ADR018 web-service compatibility (Render's edge serves Brotli/gzip, never zstd) and the m83 record.
- **Expected outcome:** browsers get gzip from the edge; the rest of the edge behaves as on v3.7.5.
- **Why now:** every Chrome/Edge/Firefox request to a web service gets an encoding ADR018 says bex never sends, and the w8/227 Brotli differences remain open, so a newer pin is also the base for lifting those.
- **Why a milestone:** the bump spans 250 upstream commits, including the Kubernetes Ingress metadata/access-log Service fields that request-log attribution uses, so it needs a real-Ingress verification, not just the compression gate.
- **Render parity task omitted:** no REST/GraphQL/MCP/dashboard surface changes; ADR018 already records gzip-only.

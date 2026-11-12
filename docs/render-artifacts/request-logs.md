# Render's HTTP request-log line — captured message and labels

Captured 2026-11-11 (w4/m325 t001) with a read-only `GET https://api.render.com/v1/logs?ownerId=…&resource=srv-…&type=request&limit=4`. It ran against two live web services in the reference Render workspace that bex's earlier Render captures use, with filtered reads by `statusCode=404` and `method=POST`. Client IPs, user agents, hosts and paths are replaced with documentation values below. The request ids, response times and sizes are as captured.

## The log object

```json
{
  "id": "817121b1-f067-41b2-8df9-…",
  "timestamp": "2026-11-11T17:40:51.240292102Z",
  "message": "clientIP=\"198.51.100.3\" requestID=\"d6a0faa9-d9c9-49f7\" responseTimeMS=116 responseBytes=340 userAgent=\"Mozilla/5.0 …\"",
  "labels": [
    { "name": "resource", "value": "srv-…" },
    { "name": "method", "value": "GET" },
    { "name": "statusCode", "value": "404" },
    { "name": "host", "value": "example.com" },
    { "name": "path", "value": "/api/orders" },
    { "name": "level", "value": "warning" },
    { "name": "type", "value": "request" }
  ]
}
```

- **Message:** `clientIP="<ip>" requestID="<id>" responseTimeMS=<int> responseBytes=<int> userAgent="<ua>"`, in that order. String values are double-quoted, numbers bare, and the response time is whole milliseconds. Every captured line (GET 200/404/500, POST 202) had all five keys.
- **Not in the message:** the method, status, host and path. They are labels.
- **Labels, in order:** `resource`, `method`, `statusCode`, `host`, `path`, `level`, `type`. There is no `requestID` label and no `instance`.
- **Level from the status class:** `info` for 2xx (and the POST 202s), `warning` for 404, `error` for 500.
- **The request id** is 18 characters, `8-4-4` hex (`303516bf-4995-4cb9`), the same value Render sends as `Rndr-Id`.
- **Static sites:** a `type=request` read of a live static site returned no lines, as [Render's docs](https://render.com/docs/logging) say ("Render does not emit logs for static sites").

## The dashboard row

[Render's logging docs](https://render.com/docs/logging) show the Log Explorer's rendering of a request line: `11:24:03 [GET] example.com/api/orders clientIP="198.51.100.3" requestID="8ebfa3c3-8929-4885" ...`, which is the time, the method, `host` + `path`, then the message. Its search examples (`status_code:/4../`, `method:/(GET|POST)/`, `/responseTimeMS=\d{3}\d+/`) match these labels and this message.

## bex (w4/m325)

bex-api renders each stored Traefik access record into this message at read time (`requestLine`, `lego/backend/internal/logs/loki.go`) and labels the line `method`, `statusCode`, `host`, `path` and `level` from the same record. The stored JSON is unchanged, since the path and host filters and the request metrics parse it. The edge's access log keeps the three request headers this needs: `Rndr-Id`, `True-Client-Ip` (the real client the client-identity plugin names) and `User-Agent`, plus `DownstreamContentSize`.

- **Lines stored before the rollout** lack the client, size and user agent, and show the keys they have.
- **The request id was 36 characters** (bex's UUID-shaped `Rndr-Id`), not Render's 18. Since w4/583 bex mints Render's `8-4-4` shape.
- **Static sites** get request lines on bex. Render has none; w4/585 asks whether to keep them (an ADR018 sign-off) or drop them.
- **Label order** is Render's: `resource`, `instance`, `method`, `statusCode`, `host`, `path`, `level`, `type`. That one order gives both captured line kinds; an app line from the same workspace read `resource`, `instance`, `level`, `type`.
- **Response time** is truncated to whole milliseconds; Render's rounding was not observable in the capture.
- **The `level` label is read, not filterable:** a `level` filter selects app lines only (w4/584).

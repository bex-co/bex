# Render free-service wake interstitial

**Captured:** 2026-07-15 PDT (2026-07-16 UTC) **Counterpart:** the public response while an idle Render Free web service wakes.

This record separates Render's documented product behavior from a live observation. Render's [Free instances documentation](https://render.com/docs/free#spinning-down-on-idle) says an idle Free web service spins down after 15 minutes, wakes on the next request, takes about a minute to return, and shows connecting browsers a loading page. The exact response contract below was captured against a sleeping public sample service; the sample URL is evidence of the observation, not a stable dependency of bex.

## Browser capture

A first `GET /` with a browser-style `Accept: text/html` to `https://juhjuhjuhgian-mileage-tracker-app-2022.onrender.com/` returned immediately:

```http
HTTP/1.1 503 Service Unavailable
Content-Type: text/html; charset=utf-8
Retry-After: 5
X-Render-Routing: hibernate-pending-wake
```

The page title was `Render - Application loading`. Its visible status copy progressed from `Incoming HTTP request detected ...` to `Service waking up ...`, then showed `Application loading`. The page did not use a meta refresh. Its script:

- issues a same-URL `HEAD` probe every 5 seconds;
- aborts an individual probe after 4 seconds;
- replaces the current location as soon as the probe status is not 503; and
- reloads the whole page after 45 seconds as a fallback.

The 5-second cadence agrees with `Retry-After: 5`. The browser response does not wait for the workload's cold start.

## API-client capture

While that same service was still waking, a request with `Accept: application/json` did not receive the HTML interstitial or a JSON error. Render held the request open through the cold start and returned the application's eventual response after approximately 19 seconds:

```http
HTTP/1.1 200 OK
Content-Type: text/html; charset=utf-8
X-Render-Origin-Server: Render
```

This confirms that Render's documented phrase "connecting browsers" is a real content-negotiated split. The origin application's content and status are not part of the interstitial contract.

## bex decision and parity

bex matches the browser behavior that matters to a visitor: an explicit, acceptable `text/html` media range receives a 503 loading page, `Retry-After: 5`, a 5-second same-URL `HEAD` probe, and a 45-second fallback reload. The request still wakes the App before the response is written.

bex deliberately preserves its existing API contract instead of holding API connections open for an unbounded workload start: `Accept: application/json`, an omitted `Accept`, and an ambiguous `*/*` receive the immediate `503 {"error":"service hibernated","retryAfter":5}` response with `Retry-After: 5`. `text/html;q=0` also selects JSON. This is a documented agent/API reliability divergence from Render, not an accidental parity claim.

Both default HTML interstitials—the maintenance page and this wake page—flow through the same responder helper for status, content type, no-store caching, and `HEAD` body suppression. Custom maintenance content keeps its separately bounded origin-fetch path.

## Mock-cluster verification

On 2026-07-15 PDT, the locally built multi-entrypoint image was loaded into the isolated kind App cluster and only the `bex-activator` Deployment was rolled. A temporary sleeping App and zero-replica Deployment were placed behind a stable audit Service.

- An `Accept: application/json` request received the immediate JSON 503 above with `Retry-After: 5`, changed the Deployment's desired replicas from 0 to 1, and stamped `app.bex.co/last-active`.
- After resetting the audit Deployment to zero replicas, a browser-style request received the HTML 503 with the 5-second probe and 45-second fallback constants, then changed desired replicas to 1.
- Once the awakened pod was Ready, moving that same audit Service from the activator to the app backend made the original host reachable with the app's 200 response. This models the Ingress backend transition while keeping the request origin stable.

The local operator manager was already unhealthy before the audit, so this proof isolates the milestone's activator behavior rather than claiming a fresh full operator reconciliation. The temporary App, Deployment, Service, and client pod were deleted afterward, and the shared activator Deployment was restored to its prior image.

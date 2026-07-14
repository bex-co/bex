/**
 * A browser always sends `Origin` on a cross-site POST, so requiring it to name
 * this dashboard rejects the cross-site request outright. Compare **hosts**,
 * not full origins: behind an ingress the server sees `http://…` while the
 * browser sends `https://…`, and `x-forwarded-host` is the host the browser
 * actually typed. No Origin at all (a scripted client) is not a browser CSRF
 * vector, but it is also not something to trust — deny.
 *
 * Shared by every server-fn that mutates state from a same-origin request:
 * `hydra-consent.ts`'s decision POST is a plain `<form>` submit, which a
 * cross-site page can fire with no CORS preflight at all — this check is its
 * only defense. `hydra-connected-agents.ts` and `kratos-sessions.ts`'s revoke
 * calls are `fetch()` JSON POSTs, which the browser's own CORS preflight
 * already blocks cross-site (these routes never answer one with a matching
 * `Access-Control-Allow-Origin`) — here the check is deliberate defense in
 * depth, not the only thing standing between an attacker and a revoke, in
 * case that CORS posture ever drifts (a proxy relaxing preflight, a future
 * route opening `Access-Control-Allow-Origin: *`).
 */
export function isSameOrigin(request: Request, url: URL): boolean {
  const origin = request.headers.get("origin");
  if (!origin) return false;
  let originHost: string;
  try {
    originHost = new URL(origin).host;
  } catch {
    return false;
  }
  const forwarded = request.headers.get("x-forwarded-host");
  return originHost === url.host || (!!forwarded && originHost === forwarded);
}

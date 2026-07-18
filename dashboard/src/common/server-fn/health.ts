import { config } from "@/config/config";

/**
 * Readiness for the dashboard pod (w1/m52, folds inbox 036): a freshly rolled
 * pod must not take traffic until it can actually reach bex-api — the old
 * readiness probe (`GET /`) renders fine even when every SSR data fetch would
 * fail, so Kubernetes handed traffic to pods that could only serve the
 * "Something went wrong" error page.
 *
 * The upstream check LATCHES on first success: once this pod has proven it can
 * reach bex-api, readiness never flips again on upstream health. A steady-state
 * bex-api outage must degrade to the app shell's inline error states (which
 * self-recover, t003) — not cascade into every dashboard pod going NotReady
 * and the whole dashboard turning into an LB 503.
 */

/** How long one upstream probe may take. The readiness probe's own
 * timeoutSeconds must exceed this. */
const UPSTREAM_TIMEOUT_MS = 1500;

let latchedReady = false;

/** Test seam: forget the latch so the pre-latch path is reachable again. */
export function resetHealthLatchForTests() {
  latchedReady = false;
}

// bex-api's REST base on the same origin SSR queries use (VITE_SSR_API_URL
// with its /graphql suffix removed — the same derivation config.apiBaseUrl
// applies to the browser URL). Static config, derived once.
const upstreamBase = (config.ssrApiUrl ?? "").replace(/\/graphql\/?$/, "");

async function upstreamReachable(fetchImpl: typeof fetch): Promise<boolean> {
  // No SSR upstream configured (local dev) — nothing to gate on.
  if (!upstreamBase) return true;
  try {
    const res = await fetchImpl(`${upstreamBase}/healthz`, {
      signal: AbortSignal.timeout(UPSTREAM_TIMEOUT_MS),
    });
    return res.ok;
  } catch {
    return false;
  }
}

/** GET /healthz — the readiness probe's answer. */
export async function readinessCheck(
  fetchImpl: typeof fetch = fetch,
): Promise<Response> {
  if (!latchedReady) latchedReady = await upstreamReachable(fetchImpl);
  return latchedReady
    ? new Response("ok", { status: 200 })
    : new Response("waiting for upstream", { status: 503 });
}

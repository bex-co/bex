// The SSR security headers, built once from the Vite config's env
// (vite.config.ts) and kept here so the policy is testable.

import { ASSETS_DIR } from "./assets-paths";

// The production attachments bucket endpoint (ADR095); override with
// VITE_AGENT_ATTACHMENTS_ORIGIN for another store.
export const DEFAULT_ATTACHMENTS_ORIGIN =
  "https://s3.eu-central-2.wasabisys.com";

/** The headers (lowercase) `buildSecurityHeaders` can emit. The response-header
 * plugin copies exactly these onto a response the route rules missed; a test
 * keeps the list equal to what the builder emits. */
export const SECURITY_HEADER_NAMES: ReadonlySet<string> = new Set([
  "x-content-type-options",
  "x-frame-options",
  "content-security-policy",
  "content-security-policy-report-only",
  "strict-transport-security",
]);

// The production bex-api's browser-facing Web Shell WebSocket (BEX_SHELL_WS_URL,
// pinned by scripts/gitops-validate.sh); override with VITE_SHELL_WS_URL.
export const DEFAULT_SHELL_WS_URL = "wss://ssh.bex.co/shell";

/** The first-party URLs the browser connects to besides the Kratos origin. All
 * come from the build env (vite.config.ts); a relative or missing one is the
 * page's own origin or unknown. */
export interface ConnectURLs {
  apiURL?: string;
  agentStreamURL?: string;
  shellWsURL?: string;
}

function originOf(url: string | undefined): string | undefined {
  if (!url) return undefined;
  try {
    return new URL(url).origin;
  } catch {
    return undefined; // a path such as "/graphql": the page's own origin
  }
}

/** The connect-src a browser needs, as far as the code reaches: the page itself,
 * Kratos, bex-api (GraphQL, REST, the agent stream and the live-log SSE), the Web
 * Shell WebSocket, the attachments bucket (presigned uploads), Stripe.js's API
 * (https://docs.stripe.com/security/guide, read 2026-11-14) and Cloudflare's
 * beacon. Undefined without an API origin: a build that cannot name it would
 * report every call, so it reports nothing. */
function narrowedConnectSources(
  kratosPublicURL: string,
  attachmentsOrigin: string,
  connect: ConnectURLs,
): string | undefined {
  const api = originOf(connect.apiURL);
  if (!api) return undefined;
  const origins = new Set(["'self'", api]);
  for (const url of [
    kratosPublicURL,
    connect.agentStreamURL,
    connect.shellWsURL || DEFAULT_SHELL_WS_URL,
    attachmentsOrigin,
  ]) {
    const origin = originOf(url);
    if (origin) origins.add(origin);
  }
  origins.add("https://api.stripe.com");
  origins.add("https://cloudflareinsights.com");
  return `connect-src ${[...origins].join(" ")}`;
}

export function buildSecurityHeaders(
  kratosPublicURL: string,
  kratosProxyTarget: string,
  attachmentsOrigin: string = DEFAULT_ATTACHMENTS_ORIGIN,
  connect: ConnectURLs = {},
): Record<string, string> {
  // Kratos's settings/login flows return a UiNodeScriptAttributes node
  // (`<script src="{KRATOS_PUBLIC_URL}/.well-known/ory/webauthn.js">`) that Ory
  // Elements injects to drive WebAuthn. A local same-origin proxy still receives
  // node URLs built from Kratos's upstream base URL, so dev must allow both
  // origins while production only exposes the browser-facing one.
  const scriptOrigins = new Set([new URL(kratosPublicURL).origin]);
  // Cloudflare Web Analytics injects its beacon <script> into the HTML at the
  // edge (the zone proxies dashboard.bex.co); without this origin the browser
  // logs a CSP violation on every full document load.
  scriptOrigins.add("https://static.cloudflareinsights.com");
  if (process.env.NODE_ENV !== "production") {
    scriptOrigins.add(new URL(kratosProxyTarget).origin);
  }

  // TanStack Start SSR injects inline hydration scripts; inline styles come from
  // Tailwind and shadcn/Radix. Outside production, connect-src also allows
  // plain-http localhost: `yarn dev:local` (dashboard/AGENTS.md's fast frontend
  // loop) points VITE_API_URL straight at local-bex.mjs's wide-open-CORS stub on
  // a different port (:8099) rather than through apiDevProxy's same-origin
  // tunnel — without this the CSP silently blocks that fetch and every page
  // hangs on "Select a workspace".
  const policy = (connectSrc: string) =>
    [
      "default-src 'self'",
      `script-src 'self' 'unsafe-inline' https://js.stripe.com ${[...scriptOrigins].join(" ")}`,
      "style-src 'self' 'unsafe-inline'",
      // Agent-session attachment thumbnails load from presigned URLs on the
      // attachments bucket's own origin (ADR095 D7), never from a bex origin.
      `img-src 'self' data: https://*.stripe.com ${new URL(attachmentsOrigin).origin}`,
      "font-src 'self'",
      connectSrc,
      "frame-src https://js.stripe.com https://hooks.stripe.com",
      "frame-ancestors 'none'",
      // No <base> element is used, so an injected one cannot re-root the page's
      // relative URLs. `form-action` is deliberately absent: Chrome applies it to
      // the redirect chain after a form POST, and the consent and device
      // decisions redirect to an OAuth client's own callback (dashboard/AGENTS.md,
      // Content-Security-Policy).
      "base-uri 'self'",
    ].join("; ");

  // Standard hardening headers for every SSR response. HSTS only in production
  // (NODE_ENV=production) — the dashboard runs over plain HTTP in dev, so setting
  // HSTS there would break the dev server.
  const production = process.env.NODE_ENV === "production";
  const headers: Record<string, string> = {
    "X-Content-Type-Options": "nosniff",
    "X-Frame-Options": "DENY",
    // connect-src https:/wss: still covers the API, auth, Stripe and Browser Web
    // Shell origins without listing them; the narrowed list below reports
    // against it until a sweep shows nothing it would refuse (w21/081).
    "Content-Security-Policy": policy(
      production
        ? "connect-src 'self' https: wss:"
        : "connect-src 'self' https: wss: http://localhost:* http://127.0.0.1:* ws://localhost:* ws://127.0.0.1:*",
    ),
  };
  if (production) {
    const narrowed = narrowedConnectSources(
      kratosPublicURL,
      attachmentsOrigin,
      connect,
    );
    if (narrowed) {
      headers["Content-Security-Policy-Report-Only"] = policy(narrowed);
    }
  }
  if (production) {
    headers["Strict-Transport-Security"] =
      "max-age=63072000; includeSubDomains";
  }
  return headers;
}

/**
 * The Nitro route rules: the security headers on every response, plus a cache
 * policy. Pages are server-rendered for the signed-in user, so they are
 * `private, no-cache` — no shared cache keeps them, and the browser revalidates
 * every load (w4/526; `no-store` would also keep them out of the back/forward
 * cache). Hashed build assets never change, so they are immutable; a miss
 * answers 404 `no-store` (`middleware/assets-miss.ts`). The two
 * app-association documents keep their short public lifetime.
 */
export function buildRouteRules(
  securityHeaders: Record<string, string>,
): Record<string, { headers: Record<string, string> }> {
  const association = {
    ...securityHeaders,
    "Content-Type": "application/json; charset=utf-8",
    "Cache-Control": "public, max-age=300",
  };
  return {
    "/**": {
      headers: { ...securityHeaders, "Cache-Control": "private, no-cache" },
    },
    [`/${ASSETS_DIR}/**`]: {
      headers: {
        ...securityHeaders,
        "Cache-Control": "public, max-age=31536000, immutable",
      },
    },
    // Apple rejects an extensionless association document served as the
    // dashboard's HTML fallback. Exact rules keep both verification resources
    // JSON while preserving the global hardening headers.
    "/.well-known/apple-app-site-association": { headers: association },
    "/.well-known/assetlinks.json": { headers: association },
  };
}

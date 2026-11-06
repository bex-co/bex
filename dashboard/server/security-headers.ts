// The SSR security headers, built once from the Vite config's env
// (vite.config.ts) and kept here so the policy is testable.

// The production attachments bucket endpoint (ADR095); override with
// VITE_AGENT_ATTACHMENTS_ORIGIN for another store.
export const DEFAULT_ATTACHMENTS_ORIGIN =
  "https://s3.eu-central-2.wasabisys.com";

export function buildSecurityHeaders(
  kratosPublicURL: string,
  kratosProxyTarget: string,
  attachmentsOrigin: string = DEFAULT_ATTACHMENTS_ORIGIN,
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

  // Standard hardening headers for every SSR response. HSTS only in production
  // (NODE_ENV=production) — the dashboard runs over plain HTTP in dev, so setting
  // HSTS there would break the dev server.
  const headers: Record<string, string> = {
    "X-Content-Type-Options": "nosniff",
    "X-Frame-Options": "DENY",
    // TanStack Start SSR injects inline hydration scripts; inline styles come from
    // Tailwind and shadcn/Radix. connect-src https:/wss: covers the API, auth,
    // and Browser Web Shell origins without hard-coding their values here.
    // Outside production, also allow plain-http localhost: `yarn dev:local`
    // (dashboard/AGENTS.md's fast frontend loop) points VITE_API_URL straight at
    // local-bex.mjs's wide-open-CORS stub on a different port (:8099) rather than
    // through apiDevProxy's same-origin tunnel — without this the CSP silently
    // blocks that fetch and every page hangs on "Select a workspace".
    "Content-Security-Policy": [
      "default-src 'self'",
      `script-src 'self' 'unsafe-inline' https://js.stripe.com ${[...scriptOrigins].join(" ")}`,
      "style-src 'self' 'unsafe-inline'",
      // Agent-session attachment thumbnails load from presigned URLs on the
      // attachments bucket's own origin (ADR095 D7), never from a bex origin.
      `img-src 'self' data: https://*.stripe.com ${new URL(attachmentsOrigin).origin}`,
      "font-src 'self'",
      process.env.NODE_ENV === "production"
        ? "connect-src 'self' https: wss:"
        : "connect-src 'self' https: wss: http://localhost:* http://127.0.0.1:* ws://localhost:* ws://127.0.0.1:*",
      "frame-src https://js.stripe.com https://hooks.stripe.com",
      "frame-ancestors 'none'",
    ].join("; "),
  };
  if (process.env.NODE_ENV === "production") {
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
 * cache). Hashed build assets never change, so they are immutable. The two
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
    "/assets/**": {
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

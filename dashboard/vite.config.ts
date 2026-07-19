import { defineConfig, loadEnv, type Plugin } from "vite";
import type { IncomingMessage } from "node:http";

import { devtools } from "@tanstack/devtools-vite";
import tsconfigPaths from "vite-tsconfig-paths";

import { tanstackStart } from "@tanstack/react-start/plugin/vite";

import viteReact from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { nitro } from "nitro/vite";

function readBody(req: IncomingMessage): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    req.on("data", (c: Buffer) => chunks.push(c));
    req.on("end", () => resolve(Buffer.concat(chunks)));
    req.on("error", reject);
  });
}

/**
 * Dev-only same-origin tunnel to the prod Kratos (README "Local dev against
 * prod"): a localhost page can't call auth.bex.co directly — Kratos's
 * CSRF/session cookies are SameSite=Lax on a different site, so the browser
 * would never send them, and prod CORS only allows dashboard.bex.co.
 * Tunneling Kratos under the dev server's own origin makes every call
 * same-site; Set-Cookie rewriting drops `Domain=bex.co` (stored host-only
 * for localhost) and `Secure` (page is plain http). Opt in with:
 *
 *   VITE_KRATOS_PUBLIC_URL=http://localhost:5173/kratos yarn dev
 *
 * A hand-rolled middleware rather than Vite's `server.proxy`: the nitro/
 * TanStack Start dev middleware registers ahead of Vite's built-in proxy
 * and swallows every request, so this must be a plugin that comes before
 * nitro in the plugins array.
 */
function rewriteKratosURL(raw: string, target: URL, publicURL: URL): string {
  try {
    const url = new URL(raw);
    if (url.origin === target.origin) {
      const publicPath = publicURL.pathname.replace(/\/$/, "");
      return `${publicURL.origin}${publicPath}${url.pathname}${url.search}${url.hash}`;
    }

    // Each dev-N Kratos is configured with its own localhost dashboard port.
    // When this checkout intentionally runs on another localhost port, keep
    // Kratos's browser UI redirects on the current dashboard origin too.
    const loopback = (host: string) =>
      host === "localhost" || host === "127.0.0.1" || host === "[::1]";
    const dashboardPath =
      url.pathname === "/" ||
      url.pathname === "/settings" ||
      url.pathname.startsWith("/auth/");
    if (loopback(target.hostname) && loopback(url.hostname) && dashboardPath) {
      return `${publicURL.origin}${url.pathname}${url.search}${url.hash}`;
    }
  } catch {
    // Non-URL strings in a flow body pass through untouched.
  }
  return raw;
}

function rewriteKratosJSON(
  value: unknown,
  target: URL,
  publicURL: URL,
): unknown {
  if (typeof value === "string") {
    return rewriteKratosURL(value, target, publicURL);
  }
  if (Array.isArray(value)) {
    return value.map((item) => rewriteKratosJSON(item, target, publicURL));
  }
  if (value && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value).map(([key, item]) => [
        key,
        rewriteKratosJSON(item, target, publicURL),
      ]),
    );
  }
  return value;
}

function kratosDevProxy(
  target = "https://auth.bex.co",
  publicBase = "http://localhost:5173/kratos",
): Plugin {
  const targetURL = new URL(target);
  const publicURL = new URL(publicBase);

  return {
    name: "bex:kratos-dev-proxy",
    apply: "serve",
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        if (!req.url?.startsWith("/kratos/")) return next();
        const url = target + req.url.slice("/kratos".length);
        void (async () => {
          const headers: Record<string, string> = {};
          for (const [k, v] of Object.entries(req.headers)) {
            // host/connection are per-hop; undici sets its own
            // content-length and negotiates (and decompresses) encoding.
            if (typeof v !== "string") continue;
            if (
              [
                "host",
                "connection",
                "content-length",
                "accept-encoding",
              ].includes(k)
            )
              continue;
            headers[k] = v;
          }
          const upstream = await fetch(url, {
            method: req.method,
            headers,
            body:
              req.method === "GET" || req.method === "HEAD"
                ? undefined
                : await readBody(req),
            redirect: "manual", // Kratos 303s must reach the browser, not be followed here
          });
          res.statusCode = upstream.status;
          upstream.headers.forEach((v, k) => {
            if (
              [
                "content-encoding",
                "content-length",
                "transfer-encoding",
                "set-cookie",
              ].includes(k)
            )
              return;
            res.setHeader(
              k,
              k === "location" ? rewriteKratosURL(v, targetURL, publicURL) : v,
            );
          });
          const cookies = upstream.headers.getSetCookie();
          if (cookies.length) {
            res.setHeader(
              "set-cookie",
              cookies.map((c) =>
                c.replace(/;\s*domain=[^;]*/gi, "").replace(/;\s*secure/gi, ""),
              ),
            );
          }
          const body = Buffer.from(await upstream.arrayBuffer());
          if (
            upstream.headers.get("content-type")?.includes("application/json")
          ) {
            try {
              const json: unknown = JSON.parse(body.toString("utf8"));
              res.end(
                JSON.stringify(rewriteKratosJSON(json, targetURL, publicURL)),
              );
              return;
            } catch {
              // Preserve an upstream body that only claimed to be JSON.
            }
          }
          res.end(body);
        })().catch((err: unknown) => {
          res.statusCode = 502;
          res.end(`kratos-dev-proxy: ${String(err)}`);
        });
      });
    },
  };
}

/**
 * Dev-only same-origin tunnel to bex-api (the API counterpart of
 * {@link kratosDevProxy}). A localhost page can't call `api.bex.co/graphql`
 * directly — prod CORS only allows `dashboard.bex.co`, and the `ory_kratos_session`
 * cookie is host-only on localhost (rewritten by the Kratos proxy on login), so a
 * cross-site call would be blocked and send no credential. Tunnelling `/graphql`
 * and `/v1/*` under the dev server's own origin makes the call same-site: the
 * browser attaches the localhost session cookie, which this forwards to bex-api
 * (auth.go reads `ory_kratos_session` and re-checks it against Kratos). The `/v1/*`
 * path also carries the live-log SSE stream, so response bodies must be streamed
 * instead of buffered. Opt in with:
 *
 *   VITE_API_URL=http://localhost:5173/graphql yarn dev
 *
 * Same middleware-before-nitro requirement as the Kratos proxy.
 */
function apiDevProxy(target = "https://api.bex.co/graphql"): Plugin {
  return {
    name: "bex:api-dev-proxy",
    apply: "serve",
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        if (req.url !== "/graphql" && !req.url?.startsWith("/v1/")) {
          return next();
        }
        void (async () => {
          const abort = new AbortController();
          res.once("close", () => abort.abort());
          const url =
            req.url === "/graphql"
              ? target
              : new URL(req.url ?? "/", new URL(target).origin).toString();
          const headers: Record<string, string> = {};
          for (const [k, v] of Object.entries(req.headers)) {
            if (typeof v !== "string") continue;
            if (
              [
                "host",
                "connection",
                "content-length",
                "accept-encoding",
              ].includes(k)
            )
              continue;
            headers[k] = v;
          }
          const upstream = await fetch(url, {
            method: req.method,
            headers,
            signal: abort.signal,
            body:
              req.method === "GET" || req.method === "HEAD"
                ? undefined
                : await readBody(req),
          });
          res.statusCode = upstream.status;
          upstream.headers.forEach((v, k) => {
            if (
              [
                "content-encoding",
                "content-length",
                "transfer-encoding",
              ].includes(k)
            )
              return;
            res.setHeader(k, v);
          });
          if (!upstream.body) {
            res.end();
            return;
          }
          const reader = upstream.body.getReader();
          try {
            while (true) {
              const { done, value } = await reader.read();
              if (done) break;
              if (!res.write(Buffer.from(value))) {
                await new Promise<void>((resolve) =>
                  res.once("drain", resolve),
                );
              }
            }
          } finally {
            reader.releaseLock();
          }
          res.end();
        })().catch((err: unknown) => {
          if (res.headersSent) {
            res.destroy();
            return;
          }
          res.statusCode = 502;
          res.end(`api-dev-proxy: ${String(err)}`);
        });
      });
    },
  };
}

function buildSecurityHeaders(
  kratosPublicURL: string,
  kratosProxyTarget: string,
): Record<string, string> {
  // Kratos's settings/login flows return a UiNodeScriptAttributes node
  // (`<script src="{KRATOS_PUBLIC_URL}/.well-known/ory/webauthn.js">`) that Ory
  // Elements injects to drive WebAuthn. A local same-origin proxy still receives
  // node URLs built from Kratos's upstream base URL, so dev must allow both
  // origins while production only exposes the browser-facing one.
  const scriptOrigins = new Set([new URL(kratosPublicURL).origin]);
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
    // (dashboard/CLAUDE.md's fast frontend loop) points VITE_API_URL straight at
    // local-bex.mjs's wide-open-CORS stub on a different port (:8099) rather than
    // through apiDevProxy's same-origin tunnel — without this the CSP silently
    // blocks that fetch and every page hangs on "Select a workspace".
    "Content-Security-Policy": [
      "default-src 'self'",
      `script-src 'self' 'unsafe-inline' ${[...scriptOrigins].join(" ")}`,
      "style-src 'self' 'unsafe-inline'",
      "img-src 'self' data:",
      "font-src 'self'",
      process.env.NODE_ENV === "production"
        ? "connect-src 'self' https: wss:"
        : "connect-src 'self' https: wss: http://localhost:* http://127.0.0.1:* ws://localhost:* ws://127.0.0.1:*",
      "frame-ancestors 'none'",
    ].join("; "),
  };
  if (process.env.NODE_ENV === "production") {
    headers["Strict-Transport-Security"] =
      "max-age=63072000; includeSubDomains";
  }
  return headers;
}

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  // Vite loads .env after config evaluation unless the config explicitly asks
  // for it. The browser-facing URLs remain same-origin while their SSR siblings
  // double as the dev proxy's direct upstream targets.
  const env = loadEnv(mode, process.cwd(), "");
  const kratosPublicURL = env.VITE_KRATOS_PUBLIC_URL || "https://auth.bex.co";
  const kratosProxyTarget = env.VITE_KRATOS_SSR_URL || "https://auth.bex.co";
  const apiProxyTarget = env.VITE_SSR_API_URL || "https://api.bex.co/graphql";
  const securityHeaders = buildSecurityHeaders(
    kratosPublicURL,
    kratosProxyTarget,
  );

  return {
    plugins: [
      kratosDevProxy(kratosProxyTarget, kratosPublicURL),
      apiDevProxy(apiProxyTarget),
      devtools(),
      tsconfigPaths({ projects: ["./tsconfig.json"] }),
      tailwindcss(),
      tanstackStart(),
      viteReact(),
      nitro({ routeRules: { "/**": { headers: securityHeaders } } }),
    ],
    ssr: {
      // @ory/elements-react ships extensionless relative imports
      // (e.g. "./session-provider") that only resolve under bundler
      // resolution, not Node's strict ESM loader — bundle it for SSR too.
      noExternal: ["@ory/elements-react"],
    },
    build: {
      assetsDir: "assets",
      sourcemap: true, // Enable source maps for better error debugging in production
      manifest: true, // Generate .vite/manifest.json for deterministic asset resolution
    },
  };
});

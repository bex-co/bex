import { defineConfig, type Plugin } from "vite";
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
function kratosDevProxy(target = "https://auth.bex.co"): Plugin {
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
            res.setHeader(k, v);
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
          res.end(Buffer.from(await upstream.arrayBuffer()));
        })().catch((err: unknown) => {
          res.statusCode = 502;
          res.end(`kratos-dev-proxy: ${String(err)}`);
        });
      });
    },
  };
}

/**
 * Dev-only same-origin tunnel to the prod bex-api GraphQL (the API counterpart
 * of {@link kratosDevProxy}). A localhost page can't call `api.bex.co/graphql`
 * directly — prod CORS only allows `dashboard.bex.co`, and the `ory_kratos_session`
 * cookie is host-only on localhost (rewritten by the Kratos proxy on login), so a
 * cross-site call would be blocked and send no credential. Tunnelling `/graphql`
 * under the dev server's own origin makes the call same-site: the browser attaches
 * the localhost session cookie, which this forwards to bex-api (auth.go reads
 * `ory_kratos_session` and re-checks it against Kratos). Opt in with:
 *
 *   VITE_API_URL=http://localhost:5173/graphql yarn dev
 *
 * Same middleware-before-nitro requirement as the Kratos proxy.
 */
function graphqlDevProxy(target = "https://api.bex.co/graphql"): Plugin {
  return {
    name: "bex:graphql-dev-proxy",
    apply: "serve",
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        if (req.url !== "/graphql") return next();
        void (async () => {
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
          const upstream = await fetch(target, {
            method: req.method,
            headers,
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
          res.end(Buffer.from(await upstream.arrayBuffer()));
        })().catch((err: unknown) => {
          res.statusCode = 502;
          res.end(`graphql-dev-proxy: ${String(err)}`);
        });
      });
    },
  };
}

// Standard hardening headers for every SSR response. HSTS only in production
// (NODE_ENV=production) — the dashboard runs over plain HTTP in dev, so setting
// HSTS there would break the dev server.
const securityHeaders: Record<string, string> = {
  "X-Content-Type-Options": "nosniff",
  "X-Frame-Options": "DENY",
  // TanStack Start SSR injects inline hydration scripts; inline styles come from
  // Tailwind and shadcn/Radix. connect-src https: covers the API and auth
  // origins (api.bex.co, auth.bex.co) without hard-coding their values here.
  // Outside production, also allow plain-http localhost: `yarn dev:local`
  // (dashboard/CLAUDE.md's fast frontend loop) points VITE_API_URL straight at
  // local-bex.mjs's wide-open-CORS stub on a different port (:8099) rather than
  // through graphqlDevProxy's same-origin tunnel — without this the CSP silently
  // blocks that fetch and every page hangs on "Select a workspace".
  "Content-Security-Policy": [
    "default-src 'self'",
    "script-src 'self' 'unsafe-inline'",
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data:",
    "font-src 'self'",
    process.env.NODE_ENV === "production"
      ? "connect-src 'self' https:"
      : "connect-src 'self' https: http://localhost:*",
    "frame-ancestors 'none'",
  ].join("; "),
};
if (process.env.NODE_ENV === "production") {
  securityHeaders["Strict-Transport-Security"] =
    "max-age=63072000; includeSubDomains";
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    kratosDevProxy(),
    graphqlDevProxy(),
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
    cssCodeSplit: false, // Bundle all CSS into one file for SSR
    sourcemap: true, // Enable source maps for better error debugging in production
    manifest: true, // Generate .vite/manifest.json for deterministic asset resolution
  },
});

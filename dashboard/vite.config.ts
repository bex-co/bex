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
            if (["host", "connection", "content-length", "accept-encoding"].includes(k))
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
              ["content-encoding", "content-length", "transfer-encoding", "set-cookie"].includes(k)
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

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    kratosDevProxy(),
    devtools(),
    tsconfigPaths({ projects: ["./tsconfig.json"] }),
    tailwindcss(),
    tanstackStart(),
    viteReact(),
    nitro(),
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

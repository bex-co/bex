import { describe, expect, it, vi } from "vitest";

import { ASSETS_DIR } from "../assets-paths";
import assetsMiss from "../middleware/assets-miss";
import {
  buildRouteRules,
  buildSecurityHeaders,
  SECURITY_HEADER_NAMES,
} from "../security-headers";

const missStatus = (path: string) =>
  assetsMiss({ url: new URL(`https://dashboard.bex.co${path}`) })?.status;

function directive(headers: Record<string, string>, name: string): string {
  const csp = headers["Content-Security-Policy"] ?? "";
  return csp.split("; ").find((d) => d.startsWith(`${name} `)) ?? "";
}

describe("buildSecurityHeaders", () => {
  it("emits exactly the headers the response-header plugin copies", () => {
    vi.stubEnv("NODE_ENV", "production");
    const emitted = Object.keys(
      buildSecurityHeaders(
        "https://auth.bex.co",
        "https://auth.bex.co",
        undefined,
        { apiURL: "https://api.bex.co/graphql" },
      ),
    )
      .map((name) => name.toLowerCase())
      .sort();
    vi.unstubAllEnvs();
    expect(emitted).toEqual([...SECURITY_HEADER_NAMES].sort());
  });

  it("lets images load from the attachments bucket and nowhere new", () => {
    const img = directive(
      buildSecurityHeaders("https://auth.bex.co", "https://auth.bex.co"),
      "img-src",
    );
    expect(img).toBe(
      "img-src 'self' data: https://*.stripe.com https://s3.eu-central-2.wasabisys.com",
    );
  });

  it("uses an overridden attachments origin, reduced to its origin", () => {
    const img = directive(
      buildSecurityHeaders(
        "https://auth.bex.co",
        "https://auth.bex.co",
        "https://store.example.com/bucket/path",
      ),
      "img-src",
    );
    expect(img).toContain(" https://store.example.com");
    expect(img).not.toContain("wasabisys");
    expect(img).not.toContain("/bucket");
  });

  // A <base> injected into markup would re-root every relative URL on the page
  // (w21/m410 t003); the app uses none, so the policy forbids them.
  it("pins the document's base URL to its own origin", () => {
    const headers = buildSecurityHeaders(
      "https://auth.bex.co",
      "https://auth.bex.co",
    );
    expect(directive(headers, "base-uri")).toBe("base-uri 'self'");
  });

  it("does not widen the other directives", () => {
    const headers = buildSecurityHeaders(
      "https://auth.bex.co",
      "https://auth.bex.co",
    );
    expect(directive(headers, "default-src")).toBe("default-src 'self'");
    expect(directive(headers, "connect-src")).not.toContain("wasabisys");
    expect(directive(headers, "script-src")).not.toContain("wasabisys");
  });

  it("allows Google Analytics origins only when GA is configured", () => {
    const off = buildSecurityHeaders(
      "https://auth.bex.co",
      "https://auth.bex.co",
    );
    expect(directive(off, "script-src")).not.toContain("google");
    expect(directive(off, "img-src")).not.toContain("google");

    const on = buildSecurityHeaders(
      "https://auth.bex.co",
      "https://auth.bex.co",
      undefined,
      { googleAnalytics: true },
    );
    expect(directive(on, "script-src")).toContain(
      " https://*.googletagmanager.com",
    );
    expect(directive(on, "img-src")).toContain(
      " https://*.google-analytics.com https://*.googletagmanager.com",
    );
  });
});

// w4/526: signed-in pages carried no Cache-Control at all.
describe("buildRouteRules", () => {
  const security = buildSecurityHeaders(
    "https://auth.bex.co",
    "https://auth.bex.co",
  );
  const rules = buildRouteRules(security);

  it("keeps pages out of shared caches and revalidated", () => {
    expect(rules["/**"].headers["Cache-Control"]).toBe("private, no-cache");
    expect(rules["/**"].headers["X-Frame-Options"]).toBe("DENY");
  });

  it("caches hashed assets and keeps the association documents' own policy", () => {
    expect(rules[`/${ASSETS_DIR}/**`].headers["Cache-Control"]).toBe(
      "public, max-age=31536000, immutable",
    );
    expect(rules[`/${ASSETS_DIR}/**`].headers["X-Frame-Options"]).toBe("DENY");
    for (const path of [
      "/.well-known/apple-app-site-association",
      "/.well-known/assetlinks.json",
    ]) {
      expect(rules[path].headers["Cache-Control"]).toBe("public, max-age=300");
      expect(rules[path].headers["Content-Type"]).toBe(
        "application/json; charset=utf-8",
      );
      expect(rules[path].headers["X-Content-Type-Options"]).toBe("nosniff");
    }
  });

  // w4/m348: a route rule's headers land on whatever answers the path, so an
  // immutable rule over a path whose miss falls through to the SSR shell
  // caches HTML under a script URL for a year. Each immutable path needs the
  // 404 no-store miss middleware (scripts/check-built-server.mjs checks the
  // build).
  it("puts immutable only on paths whose misses answer 404", () => {
    const immutable = Object.entries(rules)
      .filter(([, rule]) =>
        rule.headers["Cache-Control"]?.includes("immutable"),
      )
      .map(([path]) => path);
    expect(immutable).toEqual([`/${ASSETS_DIR}/**`]);
    for (const path of immutable) {
      expect(missStatus(path.replace("/**", "/missing-Xy12Ab.js"))).toBe(404);
    }
  });

  it("answers the retired asset directory as a miss", () => {
    expect(missStatus("/assets/vendor-react-v-sG7it2.js")).toBe(404);
    expect(rules["/assets/**"]).toBeUndefined();
  });
});

// Production enforces a connect-src that names the origins the app reaches
// (w21/083, after a QA sweep of every flow against the real origins showed no
// refusal); every other environment, and a build that cannot name its API
// origin, keeps the wide list.
describe("the enforced connect-src", () => {
  const build = (
    connect: Parameters<typeof buildSecurityHeaders>[3] = {
      apiURL: "https://api.bex.co/graphql",
    },
    attachmentsOrigin?: string,
  ) =>
    buildSecurityHeaders(
      "https://auth.bex.co",
      "https://auth.bex.co",
      attachmentsOrigin,
      connect,
    );
  const production = <T>(run: () => T): T => {
    vi.stubEnv("NODE_ENV", "production");
    try {
      return run();
    } finally {
      vi.unstubAllEnvs();
    }
  };
  const origins = (headers: Record<string, string>) =>
    directive(headers, "connect-src").split(" ").slice(1).sort();

  it("names the first-party, Stripe and Cloudflare origins in production", () => {
    const headers = production(() =>
      build({
        apiURL: "https://api.bex.co/graphql",
        agentStreamURL: "https://ssh.bex.co/agent",
      }),
    );
    expect(origins(headers)).toEqual(
      [
        "'self'",
        "https://api.bex.co",
        "https://auth.bex.co",
        "https://ssh.bex.co",
        "wss://ssh.bex.co",
        "https://api.stripe.com",
        "https://cloudflareinsights.com",
      ].sort(),
    );
    // The scheme-wide sources are what narrowing removed.
    expect(directive(headers, "connect-src")).not.toMatch(/ (https|wss):( |$)/);
  });

  it("sends no report-only policy any more", () => {
    expect(production(() => build())).not.toHaveProperty(
      "Content-Security-Policy-Report-Only",
    );
  });

  // The browser uploads an attachment through bex-api, downloads by navigating
  // to the presigned URL and shows a thumbnail as an image, so the bucket is
  // an img-src origin only, whatever it is called.
  it("keeps the attachments bucket out of connect-src and in img-src", () => {
    const headers = production(() =>
      build(
        { apiURL: "https://api.bex.co/graphql" },
        "https://store.example.com",
      ),
    );
    expect(directive(headers, "connect-src")).not.toContain(
      "store.example.com",
    );
    expect(directive(headers, "connect-src")).not.toContain("wasabisys");
    expect(directive(headers, "img-src")).toContain(
      " https://store.example.com",
    );
  });

  it("changes only connect-src against the policy production sent before", () => {
    const headers = production(() => build());
    expect(
      headers["Content-Security-Policy"]
        .split("; ")
        .filter((d) => !d.startsWith("connect-src ")),
    ).toEqual([
      "default-src 'self'",
      "script-src 'self' 'unsafe-inline' https://js.stripe.com https://auth.bex.co https://static.cloudflareinsights.com",
      "style-src 'self' 'unsafe-inline'",
      "img-src 'self' data: https://*.stripe.com https://s3.eu-central-2.wasabisys.com",
      "font-src 'self'",
      "frame-src https://js.stripe.com https://hooks.stripe.com",
      "frame-ancestors 'none'",
      "base-uri 'self'",
    ]);
  });

  it("names the Web Shell WebSocket from its override", () => {
    const headers = production(() =>
      build({
        apiURL: "https://api.bex.co/graphql",
        shellWsURL: "wss://shell.example.com/shell",
      }),
    );
    expect(origins(headers)).toContain("wss://shell.example.com");
    expect(origins(headers)).not.toContain("wss://ssh.bex.co");
  });

  it("names GA4's collect origins when GA is configured", () => {
    const on = production(() =>
      build({ apiURL: "https://api.bex.co/graphql", googleAnalytics: true }),
    );
    expect(directive(on, "connect-src")).toContain(
      " https://*.google-analytics.com https://*.analytics.google.com https://*.googletagmanager.com",
    );
    expect(
      directive(
        production(() => build()),
        "connect-src",
      ),
    ).not.toContain("google");
  });

  it("keeps the wide list when the build cannot name its API origin", () => {
    for (const connect of [{}, { apiURL: "/graphql" }]) {
      expect(
        directive(
          production(() => build(connect)),
          "connect-src",
        ),
      ).toBe("connect-src 'self' https: wss:");
    }
  });

  it("keeps the wide list, plus localhost, outside production", () => {
    expect(directive(build(), "connect-src")).toBe(
      "connect-src 'self' https: wss: http://localhost:* http://127.0.0.1:* ws://localhost:* ws://127.0.0.1:*",
    );
  });
});

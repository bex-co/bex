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

// The enforced policy still lets the page connect to any https or wss origin.
// Next to it, production reports what a connect-src narrowed to the origins the
// code reaches would refuse (w21/081), so a flow it would break shows up as a
// "[Report Only]" console violation before anything is enforced.
describe("the report-only narrowed connect-src", () => {
  const build = (connect = { apiURL: "https://api.bex.co/graphql" }) =>
    buildSecurityHeaders(
      "https://auth.bex.co",
      "https://auth.bex.co",
      undefined,
      connect,
    );
  const reportOnly = (headers: Record<string, string>) =>
    headers["Content-Security-Policy-Report-Only"] ?? "";

  it("names the first-party, Stripe, Cloudflare and attachments origins in production", () => {
    vi.stubEnv("NODE_ENV", "production");
    const headers = build({
      apiURL: "https://api.bex.co/graphql",
      agentStreamURL: "https://ssh.bex.co/agent",
      shellWsURL: "",
    } as never);
    vi.unstubAllEnvs();
    const connectSrc = (
      reportOnly(headers)
        .split("; ")
        .find((d) => d.startsWith("connect-src ")) ?? ""
    )
      .split(" ")
      .slice(1)
      .sort();
    expect(connectSrc).toEqual(
      [
        "'self'",
        "https://api.bex.co",
        "https://auth.bex.co",
        "https://ssh.bex.co",
        "wss://ssh.bex.co",
        "https://s3.eu-central-2.wasabisys.com",
        "https://api.stripe.com",
        "https://cloudflareinsights.com",
      ].sort(),
    );
  });

  it("is the enforced policy with only connect-src narrowed", () => {
    vi.stubEnv("NODE_ENV", "production");
    const headers = build();
    vi.unstubAllEnvs();
    const other = (policy: string) =>
      policy.split("; ").filter((d) => !d.startsWith("connect-src "));
    expect(other(reportOnly(headers))).toEqual(
      other(headers["Content-Security-Policy"]),
    );
    expect(directive(headers, "connect-src")).toBe(
      "connect-src 'self' https: wss:",
    );
    expect(reportOnly(headers)).not.toMatch(
      /connect-src[^;]* (https|wss):(;| |$)/,
    );
  });

  it("is absent without an API origin to name, and outside production", () => {
    vi.stubEnv("NODE_ENV", "production");
    expect(build({} as never)).not.toHaveProperty(
      "Content-Security-Policy-Report-Only",
    );
    expect(build({ apiURL: "/graphql" })).not.toHaveProperty(
      "Content-Security-Policy-Report-Only",
    );
    vi.unstubAllEnvs();
    expect(build()).not.toHaveProperty("Content-Security-Policy-Report-Only");
  });
});

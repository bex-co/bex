import { describe, expect, it } from "vitest";

import { buildRouteRules, buildSecurityHeaders } from "../security-headers";

function directive(headers: Record<string, string>, name: string): string {
  const csp = headers["Content-Security-Policy"] ?? "";
  return csp.split("; ").find((d) => d.startsWith(`${name} `)) ?? "";
}

describe("buildSecurityHeaders", () => {
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
    expect(rules["/assets/**"].headers["Cache-Control"]).toBe(
      "public, max-age=31536000, immutable",
    );
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
});

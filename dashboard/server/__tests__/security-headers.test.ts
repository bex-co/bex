import { describe, expect, it } from "vitest";

import { buildSecurityHeaders } from "../security-headers";

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

import { describe, expect, it } from "vitest";

import securityHeadersOnEveryResponse from "../plugins/security-headers-on-every-response";
import { buildRouteRules, buildSecurityHeaders } from "../security-headers";

// The route rules set the security headers on `event.res`, which h3 does not
// merge into a Response a handler returns itself: a redirect, a 401, a 404 left
// the server without CSP or X-Frame-Options (w21/m410 t003). The plugin's Nitro
// "response" hook adds them from the rules the event matched.
const rules = buildRouteRules(
  buildSecurityHeaders("https://auth.bex.co", "https://auth.bex.co"),
)["/**"].headers;

const SECURITY = [
  "x-content-type-options",
  "x-frame-options",
  "content-security-policy",
  "strict-transport-security",
];

// The shape Nitro hands a response hook for a matched `headers` rule, checked
// against the built server: the header map sits under `options`.
type RouteRules = {
  headers?: { name: string; route: string; options: Record<string, string> };
} | null;
const matched: RouteRules = {
  headers: { name: "headers", route: "/**", options: rules },
};

function respond(
  init: ResponseInit & { headers?: Record<string, string> },
  routeRules: RouteRules = matched,
): Response {
  let hook:
    | ((res: Response, event: { req: Request; context: unknown }) => void)
    | undefined;
  securityHeadersOnEveryResponse({
    hooks: {
      hook: (name: string, fn: typeof hook) => {
        if (name === "response") hook = fn;
      },
    },
  } as never);
  if (!hook) throw new Error("the plugin registered no response hook");
  const res = new Response(null, init);
  hook(res, {
    req: new Request("https://dashboard.bex.co/anything"),
    context: { routeRules: routeRules ?? undefined },
  });
  return res;
}

describe("the security headers on every response", () => {
  it.each([
    [307, { location: "/auth/login" }],
    [401, {}],
    [403, {}],
    [404, {}],
    [500, {}],
  ])(
    "a %i response gets the route rules' security headers",
    (status, headers) => {
      const res = respond({ status, headers });
      const expected = Object.entries(rules).filter(([name]) =>
        SECURITY.includes(name.toLowerCase()),
      );
      // CSP, X-Frame-Options and X-Content-Type-Options in every environment;
      // Strict-Transport-Security too in production.
      expect(expected.length).toBeGreaterThanOrEqual(3);
      for (const [name, value] of expected) {
        expect(res.headers.get(name), name).toBe(value);
      }
    },
  );

  it("does not overwrite a header the response already carries", () => {
    const res = respond({
      status: 404,
      headers: { "x-frame-options": "SAMEORIGIN" },
    });
    expect(res.headers.get("x-frame-options")).toBe("SAMEORIGIN");
    expect(res.headers.get("content-security-policy")).toContain(
      "default-src 'self'",
    );
  });

  it("copies only the security headers, not the rule's cache policy", () => {
    expect(respond({ status: 404 }).headers.get("cache-control")).toBeNull();
  });

  it("does nothing when no rule matched", () => {
    const res = respond({ status: 404 }, null);
    for (const name of SECURITY) expect(res.headers.get(name)).toBeNull();
  });
});

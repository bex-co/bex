import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-start/server", () => ({
  getRequestHeader: () => undefined,
}));

import { ssrForwardedHeaders } from "@/common/apollo/factory.server";

// w9/m450: an audit row an SSR query causes names the person's client, so
// SSR carries the browser's agent and the edge's client address with the
// session cookie.
describe("ssrForwardedHeaders", () => {
  it("carries the cookie, the user agent and the edge's client address", () => {
    const incoming: Record<string, string> = {
      cookie: "ory_kratos_session=abc",
      "user-agent": "Mozilla/5.0 Chrome/131.0.0.0",
      "true-client-ip": "198.51.100.23",
      "x-forwarded-for": "172.70.1.2",
    };
    expect(ssrForwardedHeaders((name) => incoming[name])).toEqual({
      Cookie: "ory_kratos_session=abc",
      "User-Agent": "Mozilla/5.0 Chrome/131.0.0.0",
      "True-Client-IP": "198.51.100.23",
    });
  });

  it("omits what the request did not carry", () => {
    expect(ssrForwardedHeaders(() => undefined)).toEqual({});
  });
});

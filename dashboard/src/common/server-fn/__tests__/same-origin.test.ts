import { describe, expect, it } from "vitest";
import { isSameOrigin } from "@/common/server-fn/same-origin";

const url = new URL("http://10.0.0.5:3000/auth/consent");

function request(headers: Record<string, string>) {
  return new Request(url, { method: "POST", headers });
}

// A cross-site request is refused by its Origin. `x-forwarded-host` is accepted
// because the ingress does not preserve Host (the SSR server sees the pod's
// address), but a header can only stand in for the host the Origin has to name:
// it never makes a foreign Origin acceptable (w21/m410 t002, recorded as
// accepted in dashboard/AGENTS.md).
describe("isSameOrigin", () => {
  it("accepts an Origin that names the request's own host", () => {
    expect(isSameOrigin(request({ origin: "http://10.0.0.5:3000" }), url)).toBe(
      true,
    );
  });

  it("accepts the browser's https Origin when the ingress forwards the host it typed", () => {
    expect(
      isSameOrigin(
        request({
          origin: "https://dashboard.bex.co",
          "x-forwarded-host": "dashboard.bex.co",
        }),
        url,
      ),
    ).toBe(true);
  });

  it("refuses a foreign Origin whatever host is forwarded", () => {
    expect(
      isSameOrigin(
        request({
          origin: "https://evil.example",
          "x-forwarded-host": "dashboard.bex.co",
        }),
        url,
      ),
    ).toBe(false);
  });

  it("refuses a request with no Origin or a malformed one", () => {
    expect(isSameOrigin(request({}), url)).toBe(false);
    expect(isSameOrigin(request({ origin: "not a url" }), url)).toBe(false);
    expect(
      isSameOrigin(
        request({ origin: "null", "x-forwarded-host": "dashboard.bex.co" }),
        url,
      ),
    ).toBe(false);
  });
});

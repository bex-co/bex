import { CombinedGraphQLErrors, ServerError } from "@apollo/client/errors";
import { isRetryableNetworkError, isUnauthorized } from "../error-policy";

function serverError(status: number): ServerError {
  return new ServerError(`HTTP ${status}`, {
    response: new Response(null, { status }),
    bodyText: "",
  });
}

describe("Apollo error policy", () => {
  it("refreshes only authentication failures", () => {
    expect(isUnauthorized(serverError(401))).toBe(true);
    expect(
      isUnauthorized(
        new CombinedGraphQLErrors({
          errors: [{ message: "no", extensions: { code: "UNAUTHENTICATED" } }],
        }),
      ),
    ).toBe(true);
    expect(isUnauthorized(serverError(403))).toBe(false);
  });

  it("retries only bounded transient HTTP/network classes", () => {
    for (const status of [429, 502, 503, 504]) {
      expect(isRetryableNetworkError(serverError(status))).toBe(true);
    }
    for (const status of [400, 401, 403, 500]) {
      expect(isRetryableNetworkError(serverError(status))).toBe(false);
    }
    expect(isRetryableNetworkError(new TypeError("network unavailable"))).toBe(
      true,
    );
  });
});

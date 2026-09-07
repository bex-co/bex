import { CombinedGraphQLErrors, ServerError } from "@apollo/client/errors";
import { describe, expect, it } from "vitest";
import { classifyInviteRedemptionError } from "../invite-redemption-error";

function gqlError(code: string, message = "copy may change") {
  return new CombinedGraphQLErrors({
    data: null,
    errors: [{ message, extensions: { code } }],
  });
}

function serverError(statusCode: number) {
  return new ServerError(`status ${statusCode}`, {
    response: new Response(null, { status: statusCode }),
    bodyText: "",
  });
}

describe("classifyInviteRedemptionError", () => {
  it.each([
    ["INVITE_ALREADY_ACCEPTED", "already-accepted"],
    ["INVITE_EXPIRED", "expired"],
    ["INVITE_INVALID", "terminal"],
    ["INVITE_PLAN_LIMIT", "plan-limit"],
    ["UNAUTHENTICATED", "terminal"],
    ["FORBIDDEN", "terminal"],
  ] as const)("classifies stable GraphQL code %s", (code, expected) => {
    expect(classifyInviteRedemptionError(gqlError(code))).toBe(expected);
  });

  it("never infers a terminal result from error prose", () => {
    expect(
      classifyInviteRedemptionError(
        gqlError("INTERNAL_SERVER_ERROR", "invite expired or already accepted"),
      ),
    ).toBe("ambiguous");
    expect(
      classifyInviteRedemptionError(new Error("invite already accepted")),
    ).toBe("ambiguous");
  });

  it.each([
    [400, "terminal"],
    [401, "terminal"],
    [403, "terminal"],
    [404, "ambiguous"],
    [408, "ambiguous"],
    [409, "ambiguous"],
    [429, "ambiguous"],
    [503, "ambiguous"],
  ] as const)("classifies HTTP %s as %s", (status, expected) => {
    expect(classifyInviteRedemptionError(serverError(status))).toBe(expected);
  });
});

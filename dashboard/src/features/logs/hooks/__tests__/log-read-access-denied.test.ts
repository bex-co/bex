import { ServerError } from "@apollo/client/errors";
import { describe, expect, it } from "vitest";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";
import { logReadAccessDenied } from "@/features/logs/hooks/use-older-log-pages";

function status(code: number): ServerError {
  return new ServerError(`status ${code}`, {
    response: new Response("no", { status: code }),
    bodyText: "no",
  });
}

// A log read the caller may not see is decided by bex-api's code or the
// transport status, never by its wording (w5/m130).
describe("logReadAccessDenied", () => {
  it.each([
    ["a missing resource", codedGraphQLError("NOT_FOUND")],
    [
      "a feature's own missing resource",
      codedGraphQLError("AGENT_SESSION_NOT_FOUND"),
    ],
    ["a denial", codedGraphQLError("FORBIDDEN")],
    ["an expired session (401)", status(401)],
    ["a transport 403", status(403)],
    ["a transport 404", status(404)],
  ])("denies on %s", (_case, error) => {
    expect(logReadAccessDenied(error)).toBe(true);
  });

  it.each([
    [
      "'not found' in the wording alone",
      uncodedGraphQLError("deploy not found"),
    ],
    [
      "'unauthorized' in the wording alone",
      uncodedGraphQLError("unauthorized"),
    ],
    ["an outage (500)", status(500)],
    ["no error", undefined],
  ])("does not deny on %s", (_case, error) => {
    expect(logReadAccessDenied(error)).toBe(false);
  });
});

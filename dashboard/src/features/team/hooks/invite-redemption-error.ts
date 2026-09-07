import { CombinedGraphQLErrors, ServerError } from "@apollo/client/errors";

export type InviteRedemptionFailure =
  | "already-accepted"
  | "expired"
  | "plan-limit"
  | "terminal"
  | "ambiguous";

const TERMINAL_CODES = new Set([
  "INVITE_INVALID",
  "UNAUTHENTICATED",
  "FORBIDDEN",
]);

/**
 * Classify only stable machine-readable outcomes as terminal. Unknown GraphQL,
 * transport, rate-limit, and service failures retain the capability because
 * the client cannot know whether the mutation committed.
 */
export function classifyInviteRedemptionError(
  error: unknown,
): InviteRedemptionFailure {
  if (CombinedGraphQLErrors.is(error)) {
    const codes = error.errors.flatMap((item) => {
      const code = item.extensions?.["code"];
      return typeof code === "string" ? [code] : [];
    });
    if (codes.includes("INVITE_ALREADY_ACCEPTED")) return "already-accepted";
    if (codes.includes("INVITE_EXPIRED")) return "expired";
    if (codes.includes("INVITE_PLAN_LIMIT")) return "plan-limit";
    if (codes.some((code) => TERMINAL_CODES.has(code))) return "terminal";
    return "ambiguous";
  }

  if (ServerError.is(error)) {
    // These raw HTTP statuses prove refusal before GraphQL mutation execution:
    // malformed HTTP/GraphQL request, unauthenticated, or forbidden. A 404 can
    // come from a transient router/upstream miss, and an unstructured 409 does
    // not prove the invite mutation did not commit; both remain ambiguous until
    // the server supplies a stable GraphQL invite code.
    return [400, 401, 403].includes(error.statusCode)
      ? "terminal"
      : "ambiguous";
  }

  return "ambiguous";
}

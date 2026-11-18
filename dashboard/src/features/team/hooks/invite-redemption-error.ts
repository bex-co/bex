import { CombinedGraphQLErrors, ServerError } from "@apollo/client/errors";
import { graphQLErrorCodes } from "@/common/lib/graphql-error";
import type { InviteKind } from "@/common/lib/invite-token";

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
    const codes = graphQLErrorCodes(error);
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

/** The message a failed preview or acceptance shows. An organization invite
 * (w7/083) names who to ask for a new one: an Owner, not a workspace admin. */
export function invitationErrorKey(
  error: unknown,
  kind: InviteKind = "workspace",
): string {
  const organization = kind === "organization";
  switch (classifyInviteRedemptionError(error)) {
    case "already-accepted":
      return "invites.used";
    case "expired":
      return organization ? "invites.orgExpired" : "invites.expired";
    case "plan-limit":
      return "invites.planLimit";
    case "terminal":
      return organization ? "invites.orgInvalid" : "invites.invalid";
    default:
      return "invites.retryError";
  }
}

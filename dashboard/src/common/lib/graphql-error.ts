import { CombinedGraphQLErrors } from "@apollo/client/errors";

/** Extracts the first GraphQL error message, falling back to a plain Error's. */
export function graphQLErrorMessage(err: unknown): string | null {
  if (CombinedGraphQLErrors.is(err)) return err.errors[0]?.message ?? null;
  if (err instanceof Error) return err.message;
  return null;
}

/**
 * The server's own explanation for a refused mutation, shaped for display next
 * to the field that caused it: the transport prefixes are stripped ("bad
 * request:" names an HTTP class, not anything the user can act on) and the first
 * letter is capitalized so it reads as a sentence.
 *
 * Returns "" when there is no usable message — a transport failure — which is a
 * caller's signal to fall back to its own generic copy. Showing the server's
 * reason is what turns "Couldn't add example.com" / "Couldn't invite bob@x.com"
 * into "Wildcard hostnames are not allowed" / "…is already a member" (w1/m81,
 * w1/m82); keep it here so both dialogs strip the same prefixes.
 */
export function refusalReason(err: unknown): string {
  const detail = (graphQLErrorMessage(err) ?? "")
    .replace(/^(graphql error:\s*)?bad request:\s*/i, "")
    .trim();
  return detail ? detail.charAt(0).toUpperCase() + detail.slice(1) : "";
}

/**
 * The server's own refusal reason, but only when the server actually answered.
 * Narrower than `refusalReason`, which also unwraps a plain `Error` — a
 * transport failure's "Failed to fetch" names nothing the user can act on, so
 * it must fall through to the caller's generic copy. Returns "" in that case.
 *
 * This is what a single-field edit wants (`useFieldMutation`, w6/037): those
 * mutations have no one stable error code to key on the way a create's
 * CONFLICT does, and "health check path must start with /" is the whole value
 * of showing a message at all.
 */
export function serverRefusalReason(err: unknown): string {
  return CombinedGraphQLErrors.is(err) ? refusalReason(err) : "";
}

/**
 * True when an error message names an authorization denial. The backend has no
 * stable error code for these yet, so every caller has to match the message —
 * this is the one place that does, so the case-insensitivity can't drift.
 */
export function isForbiddenError(err: Error | undefined | null): boolean {
  return err?.message.toLowerCase().includes("forbidden") ?? false;
}

/** True when any GraphQL error in Apollo's combined response has code. */
export function hasGraphQLErrorCode(err: unknown, code: string): boolean {
  return (
    CombinedGraphQLErrors.is(err) &&
    err.errors.some((item) => item.extensions?.["code"] === code)
  );
}

/**
 * True when a create mutation failed because the name is already taken in
 * scope (a workspace, a project, …) — keyed on the backend's stable
 * `extensions.code: "CONFLICT"` (`core.NewConflictError`, w6/m49) rather than
 * matching message text per resource type, so a backend copy change can't
 * silently stop a create-form's conflict handling from firing. Pair with
 * `refusalReason(err)` for the specific, resource-named text to show.
 */
export function isNameConflictError(err: unknown): boolean {
  return hasGraphQLErrorCode(err, "CONFLICT");
}

/**
 * The toast message for a create-mutation failure that might be a name
 * conflict: the backend's specific reason when it is, otherwise the caller's
 * own generic copy. w6/m49 graduated this here after four `use-create-*`
 * hooks (keyvalue, postgres, project, environment) each wrote the identical
 * `isNameConflictError(err) ? refusalReason(err) : generic` branch.
 */
export function conflictOrGenericMessage(err: unknown, generic: string): string {
  return isNameConflictError(err) ? refusalReason(err) : generic;
}

/**
 * Extracts PLAN_LIMIT error params from a GraphQL error's extensions field.
 * Returns the structured params when the first error carries code "PLAN_LIMIT";
 * returns null for any other error type or code so callers fall through to a
 * generic toast. Keying on the code (not a substring of the message) means
 * backend copy changes have zero effect on whether the plan-limit CTA shows.
 */
export function planLimitExtensions(
  err: unknown,
): { plan: string; limit: number } | null {
  if (!CombinedGraphQLErrors.is(err)) return null;
  const ext = err.errors[0]?.extensions;
  if (!ext || ext["code"] !== "PLAN_LIMIT") return null;
  return {
    plan: String(ext["plan"] ?? ""),
    limit: Number(ext["limit"] ?? 0),
  };
}

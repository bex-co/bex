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

/** True when bex-api refused the call as an authorization denial (w5/m128). */
export function isForbiddenError(err: unknown): boolean {
  return hasGraphQLErrorCode(err, "FORBIDDEN");
}

/** How a read that depends on a backing store bex-api may lack failed. */
export type RefusalKind = "unavailable" | "forbidden" | "generic";

/**
 * Classifies a failed read into the states a panel renders differently: its
 * backing store not wired (bex-api refuses with unavailableCode), an
 * authorization denial, or anything else.
 */
export function classifyRefusal(
  error: unknown,
  unavailableCode: string,
): RefusalKind | null {
  if (!error) return null;
  if (hasGraphQLErrorCode(error, unavailableCode)) return "unavailable";
  if (isForbiddenError(error)) return "forbidden";
  return "generic";
}

/** The code of every GraphQL error in Apollo's combined response. */
export function graphQLErrorCodes(err: unknown): string[] {
  if (!CombinedGraphQLErrors.is(err)) return [];
  return err.errors.flatMap((item) => {
    const code = item.extensions?.["code"];
    return typeof code === "string" ? [code] : [];
  });
}

/**
 * A missing resource, decided by bex-api's code rather than its wording
 * (w5/m130): the shared NOT_FOUND, or a feature's own `*_NOT_FOUND`, which
 * always wraps it (core.NewNotFoundError). A transport 404 is not one: bex-api
 * answers a missing resource 200 with errors, so a 404 status is an ingress or
 * proxy failure, which a page retries rather than redirects away from.
 */
export function isNotFoundError(err: unknown): boolean {
  return graphQLErrorCodes(err).some(
    (code) => code === "NOT_FOUND" || code.endsWith("_NOT_FOUND"),
  );
}

/**
 * The workspace-cap refusal's own message, which the create dialogs show as
 * is, or null for any other error (WORKSPACE_RESOURCE_LIMIT, w5/m130).
 */
export function workspaceCapMessage(err: unknown): string | null {
  if (!CombinedGraphQLErrors.is(err)) return null;
  return (
    err.errors.find(
      (item) => item.extensions?.["code"] === "WORKSPACE_RESOURCE_LIMIT",
    )?.message ?? null
  );
}

/** True when any GraphQL error in Apollo's combined response has code. */
export function hasGraphQLErrorCode(err: unknown, code: string): boolean {
  return graphQLErrorExtensions(err, code) !== null;
}

/**
 * The extensions of the first GraphQL error carrying code, or null: a coded
 * refusal's params (e.g. `field`) for a caller that words the refusal itself.
 */
export function graphQLErrorExtensions(
  err: unknown,
  code: string,
): Record<string, unknown> | null {
  if (!CombinedGraphQLErrors.is(err)) return null;
  return (
    err.errors.find((item) => item.extensions?.["code"] === code)?.extensions ??
    null
  );
}

/**
 * True when the server shed the request for throttling — either the per-caller
 * rate budget (`RATE_LIMITED`) or auth-admission overload (`AUTH_OVERLOADED`,
 * w4/m100). Keyed on extensions.code so copy changes cannot hide it.
 */
export function isThrottledError(err: unknown): boolean {
  return (
    hasGraphQLErrorCode(err, "RATE_LIMITED") ||
    hasGraphQLErrorCode(err, "AUTH_OVERLOADED")
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
 * The toast for a failed mutation: the server's own refusal when it answered
 * with one ("schedule must be a valid 5-field cron expression", a name
 * conflict, "total secret file size limit of 524288 bytes exceeded"), and the
 * caller's generic copy otherwise. Generic is right for a transport failure,
 * which has no answer to relay — so this reads only a GraphQL response, never a
 * plain `Error` the way `refusalReason` does ("Failed to fetch" names nothing
 * the user can act on; an expired session is a transport 401 the auth link
 * already redirects) — and for throttling, whose "rate limit exceeded" says
 * nothing that "please try again" doesn't.
 *
 * w6/037 put this contract on `useFieldMutation`; w1/m145 extended it to every
 * mutation catch site, and `mutation-error-toast-invariant.test.ts` keeps a
 * toast that ignores the caught error from coming back.
 */
export function mutationErrorMessage(err: unknown, generic: string): string {
  if (!CombinedGraphQLErrors.is(err) || isThrottledError(err)) return generic;
  return refusalReason(err) || generic;
}

/**
 * Extracts PLAN_LIMIT error params from a GraphQL error's extensions field.
 * Returns the structured params when an error carries code "PLAN_LIMIT";
 * returns null for any other error type or code so callers fall through to a
 * generic toast. Keying on the code (not a substring of the message) means
 * backend copy changes have zero effect on whether the plan-limit CTA shows.
 */
export function planLimitExtensions(
  err: unknown,
): { plan: string; limit: number } | null {
  const ext = graphQLErrorExtensions(err, "PLAN_LIMIT");
  if (!ext) return null;
  return {
    plan: String(ext["plan"] ?? ""),
    limit: Number(ext["limit"] ?? 0),
  };
}

import { CombinedGraphQLErrors } from "@apollo/client/errors";

/** Extracts the first GraphQL error message, falling back to a plain Error's. */
export function graphQLErrorMessage(err: unknown): string | null {
  if (CombinedGraphQLErrors.is(err)) return err.errors[0]?.message ?? null;
  if (err instanceof Error) return err.message;
  return null;
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

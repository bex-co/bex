import { CombinedGraphQLErrors } from "@apollo/client/errors";

/** Extracts the first GraphQL error message, falling back to a plain Error's. */
export function graphQLErrorMessage(err: unknown): string | null {
  if (CombinedGraphQLErrors.is(err)) return err.errors[0]?.message ?? null;
  if (err instanceof Error) return err.message;
  return null;
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

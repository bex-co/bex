import { CombinedGraphQLErrors } from "@apollo/client/errors";

/** Extracts the first GraphQL error message, falling back to a plain Error's. */
export function graphQLErrorMessage(err: unknown): string | null {
  if (CombinedGraphQLErrors.is(err)) return err.errors[0]?.message ?? null;
  if (err instanceof Error) return err.message;
  return null;
}

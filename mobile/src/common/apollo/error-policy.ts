import { CombinedGraphQLErrors, ServerError } from "@apollo/client/errors";

const retryableStatuses = new Set([429, 502, 503, 504]);

export function isUnauthorized(error: unknown): boolean {
  if (ServerError.is(error)) return error.statusCode === 401;
  return (
    CombinedGraphQLErrors.is(error) &&
    error.errors.some((item) => item.extensions?.code === "UNAUTHENTICATED")
  );
}

export function isRetryableNetworkError(error: unknown): boolean {
  if (ServerError.is(error)) return retryableStatuses.has(error.statusCode);
  return error instanceof TypeError;
}

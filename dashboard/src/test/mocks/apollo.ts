import { CombinedGraphQLErrors } from "@apollo/client/errors";

/**
 * Mock types and utilities for Apollo Client hooks in tests.
 * These helpers provide type-safe mock values without using `any`.
 */

/**
 * Mock return type for useQuery hook.
 * This provides a properly typed alternative to using `as any` in tests.
 * Based on Apollo Client v4's useQuery.Base.Result interface.
 */
export interface MockQueryResult<TData> {
  data: TData | undefined;
  loading: boolean;
  error: Error | undefined;
  called?: boolean;
  previousData?: TData;
  networkStatus?: number;
}

/**
 * Creates a mock useQuery result for loading state
 */
export function createLoadingQueryResult<TData>(): MockQueryResult<TData> {
  return {
    data: undefined,
    loading: true,
    error: undefined,
  };
}

/**
 * Creates a mock useQuery result for error state
 */
export function createErrorQueryResult<TData>(
  message: string,
): MockQueryResult<TData> {
  return {
    data: undefined,
    loading: false,
    error: new Error(message),
  };
}

/**
 * Creates a mock useQuery result for success state
 */
export function createSuccessQueryResult<TData>(
  data: TData,
): MockQueryResult<TData> {
  return {
    data,
    loading: false,
    error: undefined,
  };
}

/**
 * A bex-api refusal as Apollo surfaces it: one GraphQL error carrying code,
 * with any params flattened into its extensions as the server does. Its default
 * message is copy the server never sends, so a test built on it proves the
 * dashboard decides by the code and not by the wording.
 */
export function codedGraphQLError(
  code: string,
  params: Record<string, unknown> = {},
  message = "reworded by the server",
): CombinedGraphQLErrors {
  return new CombinedGraphQLErrors({
    errors: [{ message, extensions: { ...params, code } }],
  });
}

/**
 * A GraphQL error with no code, as bex-api answered its refusals before they
 * carried codes. A decision that still matched the wording would take it, so
 * it is the negative case for a decision made by code.
 */
export function uncodedGraphQLError(message: string): CombinedGraphQLErrors {
  return new CombinedGraphQLErrors({ errors: [{ message }] });
}

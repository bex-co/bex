import { useApolloClient } from "@apollo/client/react";
import { useCallback } from "react";

/**
 * Hook that returns a memoized function to clear the Apollo Client cache.
 * Specifically evicts the ROOT_QUERY from the cache, which effectively clears
 * all cached query results.
 *
 * @returns A memoized callback function that clears the Apollo Client cache
 * @example
 * ```tsx
 * const clearCache = useApolloCacheClear();
 * // Later, call clearCache() to clear all cached queries
 * ```
 */
export const useApolloCacheClear = () => {
  const client = useApolloClient();
  return useCallback(() => {
    client.cache.evict({ id: "ROOT_QUERY" });
  }, [client]);
};

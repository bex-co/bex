import { useCallback } from "react";
import { useApolloClient } from "@apollo/client/react";

/**
 * Drops the cached repo list, which belongs to the connection set: after a
 * connect or disconnect on a page that shows no repos (Account Settings), the
 * next page that does (the source picker, `useRepos`) reads the new set rather
 * than the cached answer for the old one (w1/m526).
 */
export function useForgetCachedRepos(): () => void {
  const client = useApolloClient();
  return useCallback(() => {
    client.cache.evict({ id: "ROOT_QUERY", fieldName: "repos" });
    client.cache.gc();
  }, [client]);
}

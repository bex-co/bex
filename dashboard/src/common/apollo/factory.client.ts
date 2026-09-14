import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { BatchHttpLink } from "@apollo/client/link/batch-http";
import { config } from "@/config/config";
import { apolloCacheConfig } from "./cache";
import { createRetryLink } from "./retry-link";
import { createAuthErrorLink } from "./auth-error-link";
import { handleUnauthenticated } from "./auth-redirect";

let clientInstance: ReturnType<typeof createApolloCsrClientImpl> | null = null;

/**
 * Why BatchHttpLink (not GraphQL aliases) for w4/m100 t002: bex-api admits
 * auth once per HTTP request, and the Metrics page historically fired 23–34
 * concurrent POSTs — colliding with the session whoami in-flight budget. A
 * batch collapses the whole page's simultaneous reads into one (or few) POSTs
 * while keeping each operation's cache key and error identity intact. Aliases
 * would couple every chart into one document so a single shed fails them all.
 */
function createTerminatingLink() {
  return new BatchHttpLink({
    uri: config.apiUrl,
    fetch,
    credentials: "include",
    // Metrics + chrome on one route fire ~20 ops; keep under the server's
    // batch cap of 32 so a page load is still one HTTP round trip.
    batchMax: 32,
    batchInterval: 10,
  });
}

/** Create an Apollo client for client-side rendering. */
function createApolloCsrClientImpl() {
  return new ApolloClient({
    link: ApolloLink.from([
      // Outermost, so it sees the final error after the retry link has had its
      // say (w3/m80 t001): a 401 on an already-mounted page re-checks the
      // session and, if it's gone, redirects to login instead of leaving a
      // dead-end error card. It never retries — a 401 is an answer, not a blip.
      createAuthErrorLink(() => void handleUnauthenticated()),
      // Retry transient read failures (w1/m52 t003) so a roll window's 502 or
      // connection reset self-heals instead of stranding an error state.
      createRetryLink(),
      createTerminatingLink(),
    ]),
    cache: new InMemoryCache(apolloCacheConfig),
  });
}

export function createApolloCsrClient() {
  if (!clientInstance) {
    clientInstance = createApolloCsrClientImpl();
  }
  return clientInstance;
}

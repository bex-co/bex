import {
  ApolloClient,
  ApolloLink,
  HttpLink,
  InMemoryCache,
} from "@apollo/client";
import { config } from "@/config/config";
import { apolloCacheConfig } from "./cache";
import { createRetryLink } from "./retry-link";
import { createAuthErrorLink } from "./auth-error-link";
import { handleUnauthenticated } from "./auth-redirect";

let clientInstance: ReturnType<typeof createApolloCsrClientImpl> | null = null;

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
      new HttpLink({
        uri: config.apiUrl,
        fetch,
        credentials: "include",
      }),
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

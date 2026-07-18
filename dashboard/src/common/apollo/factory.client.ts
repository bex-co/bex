import {
  ApolloClient,
  ApolloLink,
  HttpLink,
  InMemoryCache,
} from "@apollo/client";
import { config } from "@/config/config";
import { apolloCacheConfig } from "./cache";
import { createRetryLink } from "./retry-link";

let clientInstance: ReturnType<typeof createApolloCsrClientImpl> | null = null;

/** Create an Apollo client for client-side rendering. */
function createApolloCsrClientImpl() {
  return new ApolloClient({
    // Retry transient read failures (w1/m52 t003) so a roll window's 502 or
    // connection reset self-heals instead of stranding an error state.
    link: ApolloLink.from([
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

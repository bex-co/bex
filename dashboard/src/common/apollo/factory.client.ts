import { ApolloClient, HttpLink, InMemoryCache } from "@apollo/client";
import { config } from "@/config/config";

let clientInstance: ReturnType<typeof createApolloCsrClientImpl> | null = null;

/** Create an Apollo client for client-side rendering. */
function createApolloCsrClientImpl() {
  return new ApolloClient({
    link: new HttpLink({
      uri: config.apiUrl,
      fetch,
      credentials: "include",
    }),
    cache: new InMemoryCache(),
  });
}

export function createApolloCsrClient() {
  if (!clientInstance) {
    clientInstance = createApolloCsrClientImpl();
  }
  return clientInstance;
}

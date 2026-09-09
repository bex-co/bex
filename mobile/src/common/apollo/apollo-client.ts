import {
  ApolloClient,
  ApolloLink,
  HttpLink,
  InMemoryCache,
} from "@apollo/client";
import { RetryLink } from "@apollo/client/link/retry";
import { authManager } from "@/features/auth/auth-provider";
import { mobileConfig } from "@/features/auth/config";
import { createBoundaryLink } from "./boundary-link";
import { createAccessLink } from "./access-link";
import { createAuthLinks } from "./auth-links";
import { dataBoundary } from "./data-boundary";
import { isRetryableNetworkError } from "./error-policy";

const { authLink, refreshLink } = createAuthLinks({
  getAccessToken: () => authManager.getAccessToken(),
  forceRefresh: () => authManager.forceRefresh(),
  signOut: () => authManager.signOut(),
});

const retryLink = new RetryLink({
  delay: { initial: 300, max: 2_000, jitter: true },
  attempts: {
    max: 3,
    retryIf: (error, operation) =>
      operation.operationType === "query" && isRetryableNetworkError(error),
  },
});

export const apolloClient = new ApolloClient({
  cache: new InMemoryCache({
    typePolicies: {
      Query: { fields: { workspaces: { merge: false } } },
    },
  }),
  link: ApolloLink.from([
    createBoundaryLink(),
    refreshLink,
    retryLink,
    authLink,
    createAccessLink(),
    new HttpLink({ uri: mobileConfig.graphqlUrl }),
  ]),
  defaultOptions: {
    watchQuery: { fetchPolicy: "cache-and-network" },
    query: { fetchPolicy: "network-only" },
  },
});

dataBoundary.registerResetHandler(async () => {
  try {
    await apolloClient.clearStore();
  } catch {
    apolloClient.cache.restore({});
  }
});

import {
  ApolloClient,
  ApolloLink,
  HttpLink,
  InMemoryCache,
} from "@apollo/client";
import { SetContextLink } from "@apollo/client/link/context";
import { ErrorLink } from "@apollo/client/link/error";
import { RetryLink } from "@apollo/client/link/retry";
import { catchError, from, switchMap, throwError } from "rxjs";
import { authManager } from "@/features/auth/auth-provider";
import { mobileConfig } from "@/features/auth/config";
import { createBoundaryLink } from "./boundary-link";
import { dataBoundary } from "./data-boundary";
import { isRetryableNetworkError, isUnauthorized } from "./error-policy";

const authLink = new SetContextLink(async (context) => {
  const accessToken = await authManager.getAccessToken();
  return {
    headers: {
      ...context.headers,
      authorization: `Bearer ${accessToken}`,
    },
  };
});

const refreshLink = new ErrorLink(({ error, operation, forward }) => {
  if (!isUnauthorized(error) || operation.getContext().authRetried === true) {
    return;
  }
  operation.setContext({ authRetried: true });
  return from(authManager.forceRefresh()).pipe(
    switchMap(() => forward(operation)),
    catchError((refreshError) =>
      from(authManager.signOut()).pipe(
        switchMap(() => throwError(() => refreshError)),
      ),
    ),
  );
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

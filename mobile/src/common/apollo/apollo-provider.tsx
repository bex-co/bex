import { ApolloProvider } from "@apollo/client/react";
import type { ReactNode } from "react";
import { apolloClient } from "./apollo-client";

export function BexApolloProvider({ children }: { children: ReactNode }) {
  return <ApolloProvider client={apolloClient}>{children}</ApolloProvider>;
}

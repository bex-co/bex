import { createApolloCsrClient } from "@/common/apollo/factory.client";
import { createApolloSsrClient } from "@/common/apollo/factory.server";
import { createIsomorphicFn } from "@tanstack/react-start";

export const getClient = createIsomorphicFn()
  .client(createApolloCsrClient)
  .server(createApolloSsrClient);

import { createIsomorphicFn } from "@tanstack/react-start";
import { getUserAgentOnServer } from "./server";
import { getUserAgentOnClient } from "./client";

export const getUserAgent = createIsomorphicFn()
  .client(getUserAgentOnClient)
  .server(getUserAgentOnServer);

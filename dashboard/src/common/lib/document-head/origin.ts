import { createIsomorphicFn } from "@tanstack/react-start";
import { getDashboardOriginOnClient } from "./origin.client";
import { getDashboardOriginOnServer } from "./origin.server";

/** The active installation's origin, never a hosted-SaaS fallback. */
export const getDashboardOrigin = createIsomorphicFn()
  .server(getDashboardOriginOnServer)
  .client(getDashboardOriginOnClient);

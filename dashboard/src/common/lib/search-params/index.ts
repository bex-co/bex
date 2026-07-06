import { createIsomorphicFn } from "@tanstack/react-start";
import { getSearchParamOnServer } from "./server";
import { getSearchParamOnClient } from "./client";

export const getSearchParam = createIsomorphicFn()
  .client(getSearchParamOnClient)
  .server(getSearchParamOnServer);

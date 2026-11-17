import { createIsomorphicFn } from "@tanstack/react-start";
import { getReferrerOnServer } from "./server";
import { getReferrerOnClient } from "./client";

/**
 * The page that linked here, empty when the browser sent none. A link clicked
 * on another site is a full page load, so the server reads it from the
 * request's `Referer` header; the client reads the document's.
 */
export const getReferrer = createIsomorphicFn()
  .client(getReferrerOnClient)
  .server(getReferrerOnServer);

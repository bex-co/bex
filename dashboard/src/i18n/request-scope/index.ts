import { createIsomorphicFn } from "@tanstack/react-start";
import { getActiveI18nOnServer } from "./server";
import { getActiveI18nOnClient } from "./client";

/**
 * The i18next instance the current environment should read/render through:
 * the per-request instance on the server (so concurrent requests never share
 * mutable language state), the shared singleton on the client. Used by the
 * root provider, the root `beforeLoad`, and the document-head title/metadata
 * helpers so every SSR surface follows its own request's language (w6/m103).
 */
export const getActiveI18n = createIsomorphicFn()
  .server(getActiveI18nOnServer)
  .client(getActiveI18nOnClient);

import { createIsomorphicFn } from "@tanstack/react-start";
import { detectLanguageOnServer } from "./server";
import { detectLanguageOnClient } from "./client";

/**
 * Isomorphic language detection for the current request/page.
 * Server: URL > cookie > Accept-Language > default. Client: URL > cookie > default.
 */
export const detectLanguage = createIsomorphicFn()
  .server(detectLanguageOnServer)
  .client(detectLanguageOnClient);

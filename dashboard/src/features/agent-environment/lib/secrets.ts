// Workspace agent secrets and inline session secrets (ADR098): the client
// mirror of the server's naming rules, so a bad name is caught while typing.
// The server stays the authority; values never pass through here.

import {
  graphQLErrorCodes,
  isForbiddenError,
} from "@/common/lib/graphql-error";

/** ADR098 D3: an env var name the agent can read from a shell. */
export const SECRET_NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_]{0,127}$/;

export const RESERVED_PREFIXES = [
  "BEX_",
  "ANTHROPIC_",
  "OPENAI_",
  "CLAUDE_",
  "CODEX_",
  "GEMINI_",
  "OPENSANDBOX_",
  "GIT_",
];

export const RESERVED_NAMES = new Set([
  "GOOGLE_API_KEY",
  "GOOGLE_GEMINI_BASE_URL",
  "PATH",
  "HOME",
  "USER",
  "SHELL",
  "PWD",
  "TERM",
  "LANG",
  "HOSTNAME",
  "NODE_OPTIONS",
  "LD_PRELOAD",
  "LD_LIBRARY_PATH",
  "HTTP_PROXY",
  "HTTPS_PROXY",
  "NO_PROXY",
  "ALL_PROXY",
  "SSL_CERT_FILE",
]);

/** The server's per-value cap (maxSecretValueBytes). */
export const MAX_SECRET_VALUE_BYTES = 32 * 1024;
/** The server's per-session cap on attached secrets (maxSessionSecrets). */
export const MAX_SESSION_SECRETS = 20;

/** The locale key of what is wrong with a typed name, or null when valid. */
export function secretNameProblem(name: string): string | null {
  if (!SECRET_NAME_PATTERN.test(name))
    return "agentEnvironment.secrets.nameInvalid";
  const upper = name.toUpperCase();
  if (
    RESERVED_NAMES.has(upper) ||
    RESERVED_PREFIXES.some((prefix) => upper.startsWith(prefix))
  ) {
    return "agentEnvironment.secrets.nameReserved";
  }
  return null;
}

/** The locale key of what is wrong with a typed value, or null when valid. */
export function secretValueProblem(value: string): string | null {
  if (value === "") return "agentEnvironment.secrets.valueRequired";
  if (
    value.includes("\0") ||
    new TextEncoder().encode(value).length > MAX_SECRET_VALUE_BYTES
  ) {
    return "agentEnvironment.secrets.valueInvalid";
  }
  return null;
}

const SECRET_ERROR_KEYS: Record<string, string> = {
  AGENT_SECRET_NAME_INVALID: "agentEnvironment.secrets.nameInvalid",
  AGENT_SECRET_NAME_RESERVED: "agentEnvironment.secrets.nameReserved",
  AGENT_SECRET_VALUE_INVALID: "agentEnvironment.secrets.valueInvalid",
  AGENT_SECRET_LIMIT_EXCEEDED: "agentEnvironment.secrets.limitExceeded",
  AGENT_SECRET_NOT_FOUND: "agentEnvironment.secrets.notFound",
  AGENT_SECRETS_UNAVAILABLE: "agentEnvironment.secrets.unavailable",
};

/** The locale key for a refused secret write, by the server's stable code. */
export function secretErrorKey(err: unknown): string {
  for (const code of graphQLErrorCodes(err)) {
    const key = SECRET_ERROR_KEYS[code];
    if (key) return key;
  }
  return isForbiddenError(err)
    ? "agentEnvironment.secrets.forbidden"
    : "agentEnvironment.secrets.saveFailed";
}

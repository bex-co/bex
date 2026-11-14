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
  "MUSE_",
  "META_",
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

/**
 * Which sessions receive a workspace secret without naming it (ADR098 D9):
 * every session, sessions on one repository, or only sessions that pick it.
 */
export type AgentSecretScope = "workspace" | "repo" | "explicit";

export const AGENT_SECRET_SCOPES: readonly AgentSecretScope[] = [
  "workspace",
  "repo",
  "explicit",
];

/** A workspace agent secret as every read shows it: never its value. */
export interface AgentSecretView {
  name: string;
  scope: AgentSecretScope;
  /** The repository (owner/name, lowercased) a repo scope names. */
  repo: string | null;
}

/** The wire scope; an unknown one reads explicit, the narrowest. */
export function toSecretScope(raw: string): AgentSecretScope {
  return (AGENT_SECRET_SCOPES as readonly string[]).includes(raw)
    ? (raw as AgentSecretScope)
    : "explicit";
}

/**
 * A repository as bex-api stores a repo scope (agentsession.NormalizeRepository):
 * owner/name without a leading / or trailing .git, lowercased. Null when it is
 * not owner/name. Whitespace inside is refused too: no GitHub name has any,
 * so such a scope could never match a session.
 */
export function normalizeSecretRepo(raw: string): string | null {
  // Trimmed first: the card sends the field trimmed.
  const trimmed = raw
    .trim()
    .replace(/^\//, "")
    .trim()
    .replace(/\.git$/, "");
  const parts = trimmed.split("/");
  if (
    parts.length !== 2 ||
    parts.some((part) => part === "" || part === "." || part === "..") ||
    parts.some((part) => /[\s\\]/.test(part) || part.includes("\0"))
  ) {
    return null;
  }
  return `${parts[0]}/${parts[1]}`.toLowerCase();
}

/**
 * Whether a secret reaches a session on `repo` without the session picking
 * it: a workspace scope always, a repo scope when the repository matches
 * (case-insensitively, as the server compares them).
 */
export function secretAppliesByScope(
  secret: AgentSecretView,
  repo: string | null | undefined,
): boolean {
  if (secret.scope === "workspace") return true;
  if (secret.scope !== "repo" || !repo || !secret.repo) return false;
  // The server stores a repo scope normalized, so only the session's needs it.
  return normalizeSecretRepo(repo) === secret.repo;
}

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
  AGENT_SECRET_SCOPE_INVALID: "agentEnvironment.secrets.repoInvalid",
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

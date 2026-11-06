// The composer's session secrets (ADR098): workspace references and inline
// values, held until the create call sends them.

import type { AgentSessionSecretInput } from "@/graphql/schema-types";

export interface ComposerSecretsValue {
  /** Workspace agent secrets the session gets, by name. */
  secretNames: string[];
  /** Inline secrets for this session only; values never leave the composer except in the create call. */
  secrets: AgentSessionSecretInput[];
}

export const EMPTY_COMPOSER_SECRETS: ComposerSecretsValue = {
  secretNames: [],
  secrets: [],
};

/** The distinct names a composer value attaches. */
export function composerSecretNames(value: ComposerSecretsValue): string[] {
  return [
    ...new Set([...value.secretNames, ...value.secrets.map((s) => s.name)]),
  ].sort();
}

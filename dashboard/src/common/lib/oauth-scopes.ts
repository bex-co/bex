export const SCOPE_READ = "bex.read";
export const SCOPE_WRITE = "bex.write";
export const SCOPE_SENSITIVE = "bex.sensitive";
export const SCOPE_API_COMPAT = "bex.api";

export const GRANULAR_SCOPES = [
  SCOPE_READ,
  SCOPE_WRITE,
  SCOPE_SENSITIVE,
] as const;

export const IDENTITY_SCOPES = [
  "openid",
  "offline_access",
  "profile",
  "email",
] as const;

export const RECOGNIZED_SCOPES = [
  ...IDENTITY_SCOPES,
  ...GRANULAR_SCOPES,
  SCOPE_API_COMPAT,
] as const;

export const SCOPE_DESCRIPTION_KEYS: Record<string, string> = {
  openid: "auth.consentScopeOpenid",
  offline_access: "auth.consentScopeOfflineAccess",
  profile: "auth.consentScopeProfile",
  email: "auth.consentScopeEmail",
  [SCOPE_READ]: "auth.consentScopeRead",
  [SCOPE_WRITE]: "auth.consentScopeWrite",
  [SCOPE_SENSITIVE]: "auth.consentScopeSensitive",
  [SCOPE_API_COMPAT]: "auth.consentScopeApiCompat",
};

const recognized = new Set<string>(RECOGNIZED_SCOPES);

export function isRecognizedScope(scope: string): boolean {
  return recognized.has(scope);
}

export function hasGranularCapability(scopes: readonly string[]): boolean {
  return scopes.some(
    (scope) =>
      scope === SCOPE_READ ||
      scope === SCOPE_WRITE ||
      scope === SCOPE_SENSITIVE,
  );
}

/** Intersect requested scopes with the closed vocabulary. Third-party grants
 *  never keep the bex.api umbrella alias. */
export function grantableScopes(
  requested: readonly string[],
  platform: boolean,
): string[] {
  const kept = requested.filter((scope) => isRecognizedScope(scope));
  if (platform) return kept;
  return kept.filter((scope) => scope !== SCOPE_API_COMPAT);
}

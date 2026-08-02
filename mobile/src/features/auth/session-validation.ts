import type { StoredSession } from "./types";

type IdTokenClaims = {
  iss?: unknown;
  aud?: unknown;
  azp?: unknown;
  exp?: unknown;
  nonce?: unknown;
};

function decodeBase64Url(value: string): string {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=");
  const binary = atob(padded);
  const escaped = Array.from(
    binary,
    (character) => `%${character.charCodeAt(0).toString(16).padStart(2, "0")}`,
  ).join("");
  return decodeURIComponent(escaped);
}

export function validateIdTokenCorrelation(
  token: string,
  expected: { issuer: string; clientId: string; nonce: string; now: number },
): void {
  const parts = token.split(".");
  if (parts.length !== 3) throw new Error("invalid id token");
  let claims: IdTokenClaims;
  try {
    claims = JSON.parse(decodeBase64Url(parts[1])) as IdTokenClaims;
  } catch {
    throw new Error("invalid id token claims");
  }
  const audiences = Array.isArray(claims.aud) ? claims.aud : [claims.aud];
  const authorizedPartyIsValid =
    audiences.length === 1 || claims.azp === expected.clientId;
  if (
    claims.iss !== expected.issuer ||
    !audiences.includes(expected.clientId) ||
    !authorizedPartyIsValid ||
    claims.nonce !== expected.nonce ||
    typeof claims.exp !== "number" ||
    claims.exp <= expected.now
  ) {
    throw new Error("id token correlation failed");
  }
}

export function authResponseFailure(
  type: string,
  errorCode?: string,
): "cancelled" | "replay" | "unavailable" | null {
  if (type === "success") return null;
  if (type === "cancel" || type === "dismiss") return "cancelled";
  if (errorCode === "state_mismatch") return "replay";
  return "unavailable";
}

export function parseStoredSession(
  raw: unknown,
  expected: { issuer: string; clientId: string },
): StoredSession | null {
  if (typeof raw !== "object" || raw === null) return null;
  const session = raw as Partial<StoredSession>;
  if (
    session.version !== 1 ||
    session.issuer !== expected.issuer ||
    session.clientId !== expected.clientId ||
    typeof session.sessionId !== "string" ||
    session.sessionId.length < 16 ||
    typeof session.subject !== "string" ||
    session.subject.length === 0 ||
    typeof session.accessToken !== "string" ||
    session.accessToken.length === 0 ||
    typeof session.refreshToken !== "string" ||
    session.refreshToken.length === 0 ||
    typeof session.expiresAt !== "number" ||
    !Number.isFinite(session.expiresAt) ||
    typeof session.scope !== "string"
  ) {
    return null;
  }
  return session as StoredSession;
}

export function isExactAuthRedirect(url: string, redirectUri: string): boolean {
  return url === redirectUri || url.startsWith(`${redirectUri}?`);
}

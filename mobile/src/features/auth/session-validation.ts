import type { PendingOAuthAuthorization, StoredSession } from "./types";
import type { AuthFailureCode } from "./errors";

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

export function parsePendingOAuthAuthorization(
  raw: unknown,
  expected: { issuer: string; clientId: string; redirectUri: string },
): PendingOAuthAuthorization | null {
  if (typeof raw !== "object" || raw === null) return null;
  const pending = raw as Partial<PendingOAuthAuthorization>;
  if (
    pending.version !== 1 ||
    pending.issuer !== expected.issuer ||
    pending.clientId !== expected.clientId ||
    pending.redirectUri !== expected.redirectUri ||
    typeof pending.state !== "string" ||
    pending.state.length < 16 ||
    typeof pending.codeVerifier !== "string" ||
    pending.codeVerifier.length < 43 ||
    typeof pending.nonce !== "string" ||
    pending.nonce.length < 16 ||
    typeof pending.createdAt !== "number" ||
    !Number.isFinite(pending.createdAt)
  ) {
    return null;
  }
  return pending as PendingOAuthAuthorization;
}

export function isExactAuthRedirect(url: string, redirectUri: string): boolean {
  return url === redirectUri || url.startsWith(`${redirectUri}?`);
}

export type PendingOAuthCallbackResult =
  | { ok: true; code: string }
  | {
      ok: false;
      failure: AuthFailureCode;
      consumePending: boolean;
    };

export function validatePendingOAuthCallback(
  redirectUrl: string,
  pending: PendingOAuthAuthorization,
  now: number,
  maxAgeMs: number,
): PendingOAuthCallbackResult {
  if (!isExactAuthRedirect(redirectUrl, pending.redirectUri)) {
    return {
      ok: false,
      failure: "invalid_redirect",
      consumePending: false,
    };
  }
  if (pending.createdAt > now || now - pending.createdAt > maxAgeMs) {
    return { ok: false, failure: "replay", consumePending: true };
  }
  const callback = new URL(redirectUrl);
  const returnedState = callback.searchParams.get("state");
  if (!returnedState || returnedState !== pending.state) {
    return { ok: false, failure: "replay", consumePending: false };
  }
  const oauthError = callback.searchParams.get("error");
  if (oauthError) {
    return {
      ok: false,
      failure: oauthError === "access_denied" ? "cancelled" : "unavailable",
      consumePending: true,
    };
  }
  const code = callback.searchParams.get("code");
  if (!code) return { ok: false, failure: "replay", consumePending: true };
  return { ok: true, code };
}

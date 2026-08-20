import {
  authResponseFailure,
  isExactAuthRedirect,
  parsePendingOAuthAuthorization,
  parseStoredSession,
  validateIdTokenCorrelation,
  validatePendingOAuthCallback,
} from "../session-validation";

function encoded(value: unknown): string {
  return Buffer.from(JSON.stringify(value)).toString("base64url");
}

function idToken(claims: Record<string, unknown>): string {
  return `${encoded({ alg: "RS256" })}.${encoded(claims)}.signature`;
}

describe("native auth response validation", () => {
  const expected = {
    issuer: "https://oauth.bex.co",
    clientId: "bex-mobile",
    nonce: "nonce-a",
    now: 100,
  };

  it("binds issuer, audience, nonce, and expiration", () => {
    validateIdTokenCorrelation(
      idToken({
        iss: expected.issuer,
        aud: expected.clientId,
        nonce: expected.nonce,
        exp: 101,
      }),
      expected,
    );
    for (const claims of [
      {
        iss: "https://attacker.test",
        aud: expected.clientId,
        nonce: expected.nonce,
        exp: 101,
      },
      {
        iss: expected.issuer,
        aud: [expected.clientId, "other-client"],
        azp: "other-client",
        nonce: expected.nonce,
        exp: 101,
      },
      {
        iss: expected.issuer,
        aud: expected.clientId,
        nonce: "replayed",
        exp: 101,
      },
      {
        iss: expected.issuer,
        aud: expected.clientId,
        nonce: expected.nonce,
        exp: 99,
      },
    ]) {
      expect(() =>
        validateIdTokenCorrelation(idToken(claims), expected),
      ).toThrow();
    }
  });

  it("classifies state mismatch as replay instead of a retryable error", () => {
    expect(authResponseFailure("error", "state_mismatch")).toBe("replay");
    expect(authResponseFailure("cancel")).toBe("cancelled");
    expect(authResponseFailure("success")).toBe(null);
  });

  it("accepts only the byte-exact registered callback base", () => {
    const redirect = "https://dashboard.bex.co/oauth2redirect";
    expect(
      isExactAuthRedirect(`${redirect}?code=abc&state=state`, redirect),
    ).toBe(true);
    expect(isExactAuthRedirect("bex://oauth2redirect?code=abc", redirect)).toBe(
      false,
    );
    expect(isExactAuthRedirect(`${redirect}/attacker?code=abc`, redirect)).toBe(
      false,
    );
  });

  it("rejects stored sessions from another issuer or client", () => {
    const session = {
      version: 1 as const,
      sessionId: "session-123456789",
      subject: "identity-a",
      issuer: expected.issuer,
      clientId: expected.clientId,
      accessToken: "access",
      refreshToken: "refresh",
      expiresAt: 10_000,
      scope: "openid offline_access",
    };
    expect(parseStoredSession(session, expected)).toEqual(session);
    expect(
      parseStoredSession(
        { ...session, issuer: "https://attacker.test" },
        expected,
      ),
    ).toBe(null);
    expect(
      parseStoredSession({ ...session, clientId: "other" }, expected),
    ).toBe(null);
  });

  it("accepts only bound, well-formed pending PKCE state", () => {
    const pending = {
      version: 1 as const,
      issuer: expected.issuer,
      clientId: expected.clientId,
      redirectUri: "https://dashboard.bex.co/oauth2redirect",
      state: "state-1234567890",
      codeVerifier: "v".repeat(43),
      nonce: "nonce-1234567890",
      createdAt: 100,
    };
    const binding = {
      issuer: expected.issuer,
      clientId: expected.clientId,
      redirectUri: pending.redirectUri,
    };
    expect(parsePendingOAuthAuthorization(pending, binding)).toEqual(pending);
    expect(
      parsePendingOAuthAuthorization(
        { ...pending, redirectUri: "bex:/oauth2redirect" },
        binding,
      ),
    ).toBe(null);
    expect(
      parsePendingOAuthAuthorization(
        { ...pending, codeVerifier: "too-short" },
        binding,
      ),
    ).toBe(null);
  });

  it("accepts one fresh, exactly bound native authorization callback", () => {
    const pending = {
      version: 1 as const,
      issuer: expected.issuer,
      clientId: expected.clientId,
      redirectUri: "https://dashboard.bex.co/oauth2redirect",
      state: "state-1234567890",
      codeVerifier: "v".repeat(43),
      nonce: "nonce-1234567890",
      createdAt: 100,
    };
    expect(
      validatePendingOAuthCallback(
        `${pending.redirectUri}?code=one-time&state=${pending.state}`,
        pending,
        200,
        1_000,
      ),
    ).toEqual({ ok: true, code: "one-time" });
    expect(
      validatePendingOAuthCallback(
        `${pending.redirectUri}?code=attacker&state=wrong`,
        pending,
        200,
        1_000,
      ),
    ).toEqual({ ok: false, failure: "replay", consumePending: false });
    expect(
      validatePendingOAuthCallback(
        `${pending.redirectUri}?code=late&state=${pending.state}`,
        pending,
        1_101,
        1_000,
      ),
    ).toEqual({ ok: false, failure: "replay", consumePending: true });
  });
});

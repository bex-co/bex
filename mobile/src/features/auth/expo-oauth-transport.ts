import {
  AuthRequest,
  CodeChallengeMethod,
  ResponseType,
  exchangeCodeAsync,
  fetchDiscoveryAsync,
  fetchUserInfoAsync,
  refreshAsync,
  type DiscoveryDocument,
  type TokenResponse,
} from "expo-auth-session";
import { getRandomBytes } from "expo-crypto";
import { AuthFailure } from "./errors";
import { mobileConfig, type MobileConfig } from "./config";
import { SecurePendingOAuthStorage } from "./secure-storage";
import {
  authResponseFailure,
  validateIdTokenCorrelation,
  validatePendingOAuthCallback,
} from "./session-validation";
import type {
  OAuthTokenSet,
  OAuthTransport,
  PendingOAuthAuthorization,
  PendingOAuthStorage,
} from "./types";

const AUTHORIZATION_MAX_AGE_MS = 10 * 60 * 1000;

// OAuth scopes requested at authorize + refresh. `openid`/`offline_access` cover
// identity and refresh; `bex.read`/`bex.write` are the granular API capabilities
// bex-api mandates for every human OAuth token (docs/ADR012 §7, closed vocab
// bex.read/write/sensitive). Without them the bex-mobile token authenticates but
// every read (e.g. workspaces) fails the capability gate with insufficient_scope,
// which surfaces as the "Workspace unavailable" screen. The bex-mobile Hydra
// client is provisioned to grant exactly these (scripts/auth-bootstrap-client.sh).
// Supervision is read + operate only; bex.sensitive is intentionally not requested.
const OAUTH_SCOPES = ["openid", "offline_access", "bex.read", "bex.write"];

function randomHex(bytes = 24): string {
  return Array.from(getRandomBytes(bytes), (value) =>
    value.toString(16).padStart(2, "0"),
  ).join("");
}

function endpointOrigin(endpoint: string | undefined): string | null {
  if (!endpoint) return null;
  try {
    return new URL(endpoint).origin;
  } catch {
    return null;
  }
}

export function validateDiscovery(
  discovery: DiscoveryDocument,
  config: MobileConfig,
): void {
  const metadata = discovery.discoveryDocument;
  const endpoints = [
    discovery.authorizationEndpoint,
    discovery.tokenEndpoint,
    discovery.userInfoEndpoint,
  ];
  if (
    metadata?.issuer !== config.oauthIssuer ||
    endpoints.some(
      (endpoint) => endpointOrigin(endpoint) !== config.oauthIssuer,
    ) ||
    !discovery.authorizationEndpoint ||
    !discovery.tokenEndpoint ||
    !discovery.userInfoEndpoint ||
    !metadata.code_challenge_methods_supported?.includes(
      CodeChallengeMethod.S256,
    )
  ) {
    throw new AuthFailure("discovery");
  }
}

function normalizeTokenResponse(
  response: TokenResponse,
  subject: string,
  previousRefreshToken?: string,
): OAuthTokenSet {
  const refreshToken = response.refreshToken ?? previousRefreshToken;
  if (
    !response.accessToken ||
    !refreshToken ||
    !response.expiresIn ||
    response.expiresIn <= 0 ||
    !subject
  ) {
    throw new AuthFailure("invalid_response");
  }
  return {
    subject,
    accessToken: response.accessToken,
    refreshToken,
    expiresAt: (response.issuedAt + response.expiresIn) * 1000,
    scope: response.scope ?? "",
  };
}

function oauthErrorCode(error: unknown): string | undefined {
  if (typeof error !== "object" || error === null) return undefined;
  const record = error as Record<string, unknown>;
  for (const key of ["code", "error", "errorCode"] as const) {
    if (typeof record[key] === "string") return record[key];
  }
  return undefined;
}

export class ExpoOAuthTransport implements OAuthTransport {
  private discovery?: Promise<DiscoveryDocument>;
  private completion?: {
    redirectUrl: string;
    result: Promise<OAuthTokenSet>;
  };

  constructor(
    private readonly config = mobileConfig,
    private readonly pendingStorage: PendingOAuthStorage = new SecurePendingOAuthStorage(),
    private readonly now: () => number = Date.now,
  ) {}

  private async discover(): Promise<DiscoveryDocument> {
    this.discovery ??= fetchDiscoveryAsync(this.config.oauthIssuer).then(
      (value) => {
        validateDiscovery(value, this.config);
        return value;
      },
    );
    try {
      return await this.discovery;
    } catch (error) {
      this.discovery = undefined;
      if (error instanceof AuthFailure) throw error;
      throw new AuthFailure("offline");
    }
  }

  private async subjectFor(
    response: Pick<TokenResponse, "accessToken">,
    discovery: DiscoveryDocument,
  ): Promise<string> {
    const userInfo = await fetchUserInfoAsync(response, discovery);
    if (typeof userInfo.sub !== "string" || userInfo.sub.length === 0) {
      throw new AuthFailure("invalid_response");
    }
    return userInfo.sub;
  }

  async authorize(): Promise<OAuthTokenSet> {
    const discovery = await this.discover();
    const nonce = randomHex();
    const state = randomHex();
    const request = new AuthRequest({
      clientId: this.config.oauthClientId,
      redirectUri: this.config.oauthRedirectUri,
      responseType: ResponseType.Code,
      scopes: OAUTH_SCOPES,
      usePKCE: true,
      codeChallengeMethod: CodeChallengeMethod.S256,
      state,
      extraParams: { nonce, audience: this.config.oauthAudience },
    });
    const authorizationUrl = await request.makeAuthUrlAsync(discovery);
    if (!request.codeVerifier) throw new AuthFailure("unavailable");
    const pending: PendingOAuthAuthorization = {
      version: 1,
      issuer: this.config.oauthIssuer,
      clientId: this.config.oauthClientId,
      redirectUri: this.config.oauthRedirectUri,
      state: request.state,
      codeVerifier: request.codeVerifier,
      nonce,
      createdAt: this.now(),
    };
    this.completion = undefined;
    await this.pendingStorage.save(pending);

    let result: Awaited<ReturnType<AuthRequest["promptAsync"]>>;
    try {
      result = await request.promptAsync(discovery, { url: authorizationUrl });
    } catch (error) {
      await this.clearPendingIfStateMatches(request.state);
      throw error;
    }
    const resultFailure = authResponseFailure(
      result.type,
      "error" in result ? result.error?.code : undefined,
    );
    if (resultFailure) {
      await this.clearPendingIfStateMatches(request.state);
      throw new AuthFailure(resultFailure);
    }
    if (result.type !== "success") {
      await this.clearPendingIfStateMatches(request.state);
      throw new AuthFailure("unavailable");
    }
    return this.completeAuthorization(result.url);
  }

  /**
   * Converges the two observers of one native callback (the active AuthSession
   * and the Expo Router deep link both call this with the same URL) onto ONE
   * code exchange — the second observer must not re-run exchangeRedirect,
   * which would consume-fail the already-spent pending state. The memo is
   * authorization-scoped, NOT session-scoped: authorize() clears it at the
   * start of a new login and reset() clears it at every terminal session
   * boundary (sign-out, forced clear), so a replayed deep link after logout
   * cannot resurrect the old token set (round-11 #10) — it falls through to
   * exchangeRedirect, whose pending-state/PKCE checks reject it.
   */
  completeAuthorization(redirectUrl: string): Promise<OAuthTokenSet> {
    if (this.completion?.redirectUrl === redirectUrl) {
      return this.completion.result;
    }
    const result = this.exchangeRedirect(redirectUrl);
    this.completion = { redirectUrl, result };
    return result;
  }

  /** OAuthTransport.reset — see the interface doc. */
  reset(): void {
    this.completion = undefined;
  }

  private async exchangeRedirect(redirectUrl: string): Promise<OAuthTokenSet> {
    const pending = await this.pendingStorage.load();
    if (!pending) throw new AuthFailure("replay");
    const callback = validatePendingOAuthCallback(
      redirectUrl,
      pending,
      this.now(),
      AUTHORIZATION_MAX_AGE_MS,
    );
    if (!callback.ok) {
      // A mismatched deep link must not destroy a legitimate request. Expired,
      // malformed-but-bound, and provider-error callbacks are terminal.
      if (callback.consumePending) {
        await this.pendingStorage.clear().catch(() => undefined);
      }
      throw new AuthFailure(callback.failure);
    }
    const code = callback.code;

    const discovery = await this.discover();
    let token: TokenResponse;
    try {
      token = await exchangeCodeAsync(
        {
          clientId: this.config.oauthClientId,
          code,
          redirectUri: this.config.oauthRedirectUri,
          extraParams: { code_verifier: pending.codeVerifier },
        },
        discovery,
      );
    } catch (error) {
      throw new AuthFailure(
        oauthErrorCode(error) === "invalid_grant" ? "replay" : "unavailable",
      );
    } finally {
      // Authorization codes are single-use. Retaining their verifier after an
      // attempted exchange only enlarges the replay window.
      await this.pendingStorage.clear().catch(() => undefined);
    }
    if (!token.idToken) throw new AuthFailure("invalid_response");
    try {
      validateIdTokenCorrelation(token.idToken, {
        issuer: this.config.oauthIssuer,
        clientId: this.config.oauthClientId,
        nonce: pending.nonce,
        now: Math.floor(this.now() / 1000),
      });
    } catch {
      throw new AuthFailure("invalid_response");
    }
    const subject = await this.subjectFor(token, discovery);
    return normalizeTokenResponse(token, subject);
  }

  private async clearPendingIfStateMatches(state: string): Promise<void> {
    const pending = await this.pendingStorage.load().catch(() => null);
    if (pending?.state === state) {
      await this.pendingStorage.clear().catch(() => undefined);
    }
  }

  async refresh(refreshToken: string): Promise<OAuthTokenSet> {
    const discovery = await this.discover();
    try {
      const token = await refreshAsync(
        {
          clientId: this.config.oauthClientId,
          refreshToken,
          scopes: OAUTH_SCOPES,
        },
        discovery,
      );
      const subject = await this.subjectFor(token, discovery);
      return normalizeTokenResponse(token, subject, refreshToken);
    } catch (error) {
      if (error instanceof AuthFailure) throw error;
      throw new AuthFailure(
        oauthErrorCode(error) === "invalid_grant" ? "invalid_grant" : "offline",
      );
    }
  }

  async revoke(accessToken: string): Promise<void> {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 8_000);
    try {
      const response = await fetch(`${this.config.apiOrigin}/v1/oauth/revoke`, {
        method: "POST",
        headers: { Authorization: `Bearer ${accessToken}` },
        signal: controller.signal,
      });
      if (response.status !== 204 && response.status !== 401) {
        throw new AuthFailure("unavailable");
      }
    } catch (error) {
      if (error instanceof AuthFailure) throw error;
      throw new AuthFailure("offline");
    } finally {
      clearTimeout(timeout);
    }
  }
}

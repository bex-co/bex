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
import {
  authResponseFailure,
  isExactAuthRedirect,
  validateIdTokenCorrelation,
} from "./session-validation";
import type { OAuthTokenSet, OAuthTransport } from "./types";

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

  constructor(private readonly config = mobileConfig) {}

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
    const request = new AuthRequest({
      clientId: this.config.oauthClientId,
      redirectUri: this.config.oauthRedirectUri,
      responseType: ResponseType.Code,
      scopes: ["openid", "offline_access"],
      usePKCE: true,
      codeChallengeMethod: CodeChallengeMethod.S256,
      extraParams: { nonce, audience: this.config.oauthAudience },
    });
    const result = await request.promptAsync(discovery);
    const resultFailure = authResponseFailure(
      result.type,
      "error" in result ? result.error?.code : undefined,
    );
    if (resultFailure) throw new AuthFailure(resultFailure);
    if (result.type !== "success") throw new AuthFailure("unavailable");
    if (!isExactAuthRedirect(result.url, this.config.oauthRedirectUri)) {
      throw new AuthFailure("invalid_redirect");
    }
    const code = result.params.code;
    if (!code || !request.codeVerifier) throw new AuthFailure("replay");
    let token: TokenResponse;
    try {
      token = await exchangeCodeAsync(
        {
          clientId: this.config.oauthClientId,
          code,
          redirectUri: this.config.oauthRedirectUri,
          extraParams: { code_verifier: request.codeVerifier },
        },
        discovery,
      );
    } catch (error) {
      throw new AuthFailure(
        oauthErrorCode(error) === "invalid_grant" ? "replay" : "unavailable",
      );
    }
    if (!token.idToken) throw new AuthFailure("invalid_response");
    try {
      validateIdTokenCorrelation(token.idToken, {
        issuer: this.config.oauthIssuer,
        clientId: this.config.oauthClientId,
        nonce,
        now: Math.floor(Date.now() / 1000),
      });
    } catch {
      throw new AuthFailure("invalid_response");
    }
    const subject = await this.subjectFor(token, discovery);
    return normalizeTokenResponse(token, subject);
  }

  async refresh(refreshToken: string): Promise<OAuthTokenSet> {
    const discovery = await this.discover();
    try {
      const token = await refreshAsync(
        {
          clientId: this.config.oauthClientId,
          refreshToken,
          scopes: ["openid", "offline_access"],
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

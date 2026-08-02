export type StoredSession = {
  version: 1;
  sessionId: string;
  subject: string;
  issuer: string;
  clientId: string;
  accessToken: string;
  refreshToken: string;
  expiresAt: number;
  scope: string;
};

export type OAuthTokenSet = {
  subject: string;
  accessToken: string;
  refreshToken: string;
  expiresAt: number;
  scope: string;
};

/**
 * Short-lived PKCE material needed to finish a browser authorization after the
 * OS recreates the app. It is sensitive even though it is not a token, so the
 * production implementation stores it only in the device-bound secure store.
 */
export type PendingOAuthAuthorization = {
  version: 1;
  issuer: string;
  clientId: string;
  redirectUri: string;
  state: string;
  codeVerifier: string;
  nonce: string;
  createdAt: number;
};

export type AuthState =
  | { status: "loading" }
  | { status: "signedOut" }
  | { status: "expired" }
  | { status: "signedIn"; session: StoredSession };

export interface AuthStorage {
  load(): Promise<StoredSession | null>;
  save(session: StoredSession): Promise<void>;
  clear(): Promise<void>;
}

export interface PendingOAuthStorage {
  load(): Promise<PendingOAuthAuthorization | null>;
  save(value: PendingOAuthAuthorization): Promise<void>;
  clear(): Promise<void>;
}

export interface OAuthTransport {
  authorize(): Promise<OAuthTokenSet>;
  completeAuthorization(redirectUrl: string): Promise<OAuthTokenSet>;
  refresh(refreshToken: string): Promise<OAuthTokenSet>;
  revoke(accessToken: string): Promise<void>;
}

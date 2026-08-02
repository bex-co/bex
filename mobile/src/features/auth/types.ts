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

export interface OAuthTransport {
  authorize(): Promise<OAuthTokenSet>;
  refresh(refreshToken: string): Promise<OAuthTokenSet>;
  revoke(accessToken: string): Promise<void>;
}

import { AuthFailure, authFailureCode } from "./errors";
import type { MobileConfig } from "./config";
import type {
  AuthState,
  AuthStorage,
  OAuthTokenSet,
  OAuthTransport,
  StoredSession,
} from "./types";

type Listener = (state: AuthState) => void;

export class SessionManager {
  private state: AuthState = { status: "loading" };
  private current: StoredSession | null = null;
  private listeners = new Set<Listener>();
  private refreshPromise?: Promise<StoredSession>;

  constructor(
    private readonly storage: AuthStorage,
    private readonly transport: OAuthTransport,
    private readonly config: MobileConfig,
    private readonly now: () => number,
    private readonly newSessionId: () => string,
    private readonly onBoundaryReset: () => Promise<void> = async () => {},
  ) {}

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    listener(this.state);
    return () => this.listeners.delete(listener);
  }

  getState(): AuthState {
    return this.state;
  }

  private setState(state: AuthState): void {
    this.state = state;
    for (const listener of this.listeners) listener(state);
  }

  private toStored(tokens: OAuthTokenSet, sessionId: string): StoredSession {
    return {
      version: 1,
      sessionId,
      subject: tokens.subject,
      issuer: this.config.oauthIssuer,
      clientId: this.config.oauthClientId,
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
      expiresAt: tokens.expiresAt,
      scope: tokens.scope,
    };
  }

  private isFresh(session: StoredSession): boolean {
    return session.expiresAt > this.now() + 30_000;
  }

  async restore(): Promise<void> {
    try {
      const session = await this.storage.load();
      await this.onBoundaryReset();
      if (!session) {
        this.current = null;
        this.setState({ status: "signedOut" });
        return;
      }
      this.current = session;
      await this.forceRefresh();
    } catch (error) {
      if (authFailureCode(error) === "invalid_grant") {
        await this.clearLocalSession();
        return;
      }
      this.setState({ status: "expired" });
    }
  }

  async signIn(): Promise<void> {
    const tokens = await this.transport.authorize();
    const session = this.toStored(tokens, this.newSessionId());
    try {
      await this.storage.save(session);
    } catch {
      await this.transport.revoke(tokens.accessToken).catch(() => undefined);
      throw new AuthFailure("storage");
    }
    await this.onBoundaryReset();
    this.current = session;
    this.setState({ status: "signedIn", session });
  }

  async getAccessToken(): Promise<string> {
    const session = this.current;
    if (!session || this.state.status === "signedOut") {
      throw new AuthFailure("invalid_grant");
    }
    if (this.isFresh(session)) return session.accessToken;
    return (await this.forceRefresh()).accessToken;
  }

  async forceRefresh(): Promise<StoredSession> {
    this.refreshPromise ??= this.refreshCurrent().finally(() => {
      this.refreshPromise = undefined;
    });
    return this.refreshPromise;
  }

  private async refreshCurrent(): Promise<StoredSession> {
    const previous = this.current;
    if (!previous) throw new AuthFailure("invalid_grant");
    try {
      const tokens = await this.transport.refresh(previous.refreshToken);
      if (tokens.subject !== previous.subject) {
        throw new AuthFailure("invalid_response");
      }
      const session = this.toStored(tokens, previous.sessionId);
      await this.storage.save(session);
      this.current = session;
      this.setState({ status: "signedIn", session });
      return session;
    } catch (error) {
      const code = authFailureCode(error);
      if (
        code === "invalid_grant" ||
        code === "invalid_response" ||
        code === "storage"
      ) {
        await this.clearLocalSession();
      } else {
        this.setState({ status: "expired" });
      }
      throw error;
    }
  }

  async signOut(): Promise<void> {
    const current = this.current;
    const revoke = current
      ? this.transport.revoke(current.accessToken).catch(() => undefined)
      : Promise.resolve();
    await this.clearLocalSession();
    await revoke;
  }

  private async clearLocalSession(): Promise<void> {
    this.current = null;
    this.setState({ status: "loading" });
    await this.storage.clear().catch(() => undefined);
    await this.onBoundaryReset();
    this.setState({ status: "signedOut" });
  }
}

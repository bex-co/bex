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
type SessionClearHook = (session: StoredSession | null) => Promise<void> | void;

export class SessionManager {
  private state: AuthState = { status: "loading" };
  private current: StoredSession | null = null;
  private listeners = new Set<Listener>();
  private refreshPromise?: Promise<StoredSession>;
  private establishPromise?: Promise<void>;
  private sessionClearHooks = new Set<SessionClearHook>();
  /**
   * Monotonic auth epoch. Bumped the instant sign-out begins clearing state, so
   * a token refresh that started under an earlier epoch can detect that the
   * session it belonged to is gone and refuse to resurrect it (see
   * refreshCurrent). Never decremented.
   */
  private authGeneration = 0;

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

  registerSessionClearHook(hook: SessionClearHook): () => void {
    this.sessionClearHooks.add(hook);
    return () => this.sessionClearHooks.delete(hook);
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
    await this.establishSession(tokens);
  }

  async completeSignIn(redirectUrl: string): Promise<void> {
    const tokens = await this.transport.completeAuthorization(redirectUrl);
    await this.establishSession(tokens);
  }

  private async establishSession(tokens: OAuthTokenSet): Promise<void> {
    // The active AuthSession and Expo Router can both observe the same native
    // callback. They intentionally converge here; persist and publish it once.
    this.establishPromise ??= this.persistNewSession(tokens).finally(() => {
      this.establishPromise = undefined;
    });
    await this.establishPromise;
  }

  private async persistNewSession(tokens: OAuthTokenSet): Promise<void> {
    if (
      this.current?.accessToken === tokens.accessToken &&
      this.state.status === "signedIn"
    ) {
      return;
    }
    const previous = this.current;
    if (previous) {
      this.authGeneration += 1;
      await this.runSessionClearHooks(previous);
    }
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
    // Snapshot the epoch + identity this refresh belongs to. If sign-out (or a
    // fresh sign-in) lands during any await below, `superseded()` turns true and
    // the result is discarded — otherwise a token endpoint resolving after
    // logout would save tokens, restore `this.current`, and re-publish
    // `signedIn`, re-authenticating a user who signed out.
    const generation = this.authGeneration;
    const previous = this.current;
    if (!previous) throw new AuthFailure("invalid_grant");
    const superseded = () =>
      this.authGeneration !== generation || this.current !== previous;
    try {
      const tokens = await this.transport.refresh(previous.refreshToken);
      if (superseded()) return previous; // stale success — touch nothing
      if (tokens.subject !== previous.subject) {
        throw new AuthFailure("invalid_response");
      }
      const session = this.toStored(tokens, previous.sessionId);
      await this.storage.save(session);
      if (superseded()) {
        // Logout completed during the write — scrub the token we just persisted
        // so it cannot outlive the session, then discard.
        await this.storage.clear().catch(() => undefined);
        return previous;
      }
      this.current = session;
      this.setState({ status: "signedIn", session });
      return session;
    } catch (error) {
      // A stale FAILURE must not clobber newer state either: after logout (or a
      // re-login) the world moved on, so leave storage and published state as
      // the newer path left them.
      if (superseded()) throw error;
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
    // Every terminal path crosses the same boundary. This includes invalid_grant,
    // invalid_response, and storage failures as well as explicit sign-out, so
    // notification subscriptions/inboxes cannot survive a forced account clear.
    this.authGeneration += 1;
    const previous = this.current;
    this.current = null;
    this.setState({ status: "loading" });
    const featureCleanup = this.runSessionClearHooks(previous);
    await this.storage.clear().catch(() => undefined);
    await this.onBoundaryReset();
    this.setState({ status: "signedOut" });
    await featureCleanup;
  }

  private async runSessionClearHooks(
    session: StoredSession | null,
  ): Promise<void> {
    await Promise.all(
      [...this.sessionClearHooks].map((hook) =>
        Promise.resolve(hook(session)).catch(() => undefined),
      ),
    );
  }
}

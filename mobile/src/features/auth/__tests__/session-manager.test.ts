import { AuthFailure } from "../errors";
import { SessionManager } from "../session-manager";
import type {
  AuthStorage,
  OAuthTokenSet,
  OAuthTransport,
  StoredSession,
} from "../types";
import type { MobileConfig } from "../config";

const config: MobileConfig = {
  apiOrigin: "https://api.bex.co",
  graphqlUrl: "https://api.bex.co/graphql",
  oauthIssuer: "https://oauth.bex.co",
  oauthClientId: "bex-mobile",
  oauthAudience: "https://api.bex.co/mcp",
  oauthRedirectUri: "co.bex.mobile:/oauth2redirect",
};

function tokens(accessToken: string, expiresAt: number): OAuthTokenSet {
  return {
    subject: "identity-a",
    accessToken,
    refreshToken: `refresh-${accessToken}`,
    expiresAt,
    scope: "openid offline_access",
  };
}

class MemoryStorage implements AuthStorage {
  value: StoredSession | null = null;
  clears = 0;
  saveError?: Error;
  async load() {
    return this.value;
  }
  async save(value: StoredSession) {
    if (this.saveError) throw this.saveError;
    this.value = value;
  }
  async clear() {
    this.value = null;
    this.clears += 1;
  }
}

class FakeTransport implements OAuthTransport {
  authorizeResult = tokens("access-a", 120_000);
  refreshResult = tokens("access-b", 240_000);
  refreshError?: Error;
  revokeError?: Error;
  revokeGate?: Promise<void>;
  refreshGate?: Promise<void>;
  refreshCalls = 0;
  revokeCalls = 0;
  async authorize() {
    return this.authorizeResult;
  }
  async completeAuthorization() {
    return this.authorizeResult;
  }
  async refresh() {
    this.refreshCalls += 1;
    // A test-controlled pause point so a sign-out can deterministically race a
    // refresh that is parked at the token endpoint. Undefined ⇒ resolves now.
    await this.refreshGate;
    if (this.refreshError) throw this.refreshError;
    await Promise.resolve();
    return this.refreshResult;
  }
  async revoke() {
    this.revokeCalls += 1;
    await this.revokeGate;
    if (this.revokeError) throw this.revokeError;
  }
}

function manager(
  storage: MemoryStorage,
  transport: FakeTransport,
  now: () => number,
  onBoundaryReset: () => Promise<void> = async () => {},
) {
  return new SessionManager(
    storage,
    transport,
    config,
    now,
    () => "session-123456789",
    onBoundaryReset,
  );
}

describe("SessionManager", () => {
  it("stores sign-in and serializes concurrent refresh", async () => {
    let now = 0;
    const storage = new MemoryStorage();
    const transport = new FakeTransport();
    const subject = manager(storage, transport, () => now);
    await subject.signIn();
    expect(subject.getState().status).toBe("signedIn");
    now = 100_000;
    const [first, second] = await Promise.all([
      subject.getAccessToken(),
      subject.getAccessToken(),
    ]);
    expect(first).toBe("access-b");
    expect(second).toBe("access-b");
    expect(transport.refreshCalls).toBe(1);
    expect(storage.value?.accessToken).toBe("access-b");
  });

  it("establishes a session from a native OAuth callback", async () => {
    const storage = new MemoryStorage();
    const transport = new FakeTransport();
    const subject = manager(storage, transport, () => 0);
    await subject.restore();

    await subject.completeSignIn(
      "co.bex.mobile:/oauth2redirect?code=one-time&state=bound",
    );

    expect(subject.getState().status).toBe("signedIn");
    expect(storage.value?.accessToken).toBe("access-a");
  });

  it("clears a replayed or revoked refresh chain", async () => {
    const storage = new MemoryStorage();
    const transport = new FakeTransport();
    const subject = manager(storage, transport, () => 0);
    await subject.signIn();
    let clearedSession = "";
    subject.registerSessionClearHook((session) => {
      clearedSession = session?.sessionId ?? "";
    });
    transport.refreshError = new AuthFailure("invalid_grant");
    let failed = false;
    try {
      await subject.forceRefresh();
    } catch {
      failed = true;
    }
    expect(failed).toBe(true);
    expect(subject.getState().status).toBe("signedOut");
    expect(storage.value).toBe(null);
    expect(clearedSession).not.toBe("");
  });

  it("revokes the issued token when secure storage denies sign-in", async () => {
    const storage = new MemoryStorage();
    storage.saveError = new Error("device policy denied storage");
    const transport = new FakeTransport();
    const subject = manager(storage, transport, () => 0);
    await subject.restore();

    let code = "";
    try {
      await subject.signIn();
    } catch (error) {
      code = error instanceof AuthFailure ? error.code : "unexpected";
    }

    expect(code).toBe("storage");
    expect(transport.revokeCalls).toBe(1);
    expect(subject.getState().status).toBe("signedOut");
    expect(storage.value).toBe(null);
  });

  it("clears the session if refresh resolves to another identity", async () => {
    const storage = new MemoryStorage();
    const transport = new FakeTransport();
    const subject = manager(storage, transport, () => 0);
    await subject.signIn();
    transport.refreshResult = {
      ...transport.refreshResult,
      subject: "identity-b",
    };

    await subject.forceRefresh().catch(() => undefined);

    expect(subject.getState().status).toBe("signedOut");
    expect(storage.value).toBe(null);
  });

  it("fails closed but retains a refresh token during a network outage", async () => {
    const storage = new MemoryStorage();
    const transport = new FakeTransport();
    const subject = manager(storage, transport, () => 0);
    await subject.signIn();
    transport.refreshError = new AuthFailure("offline");
    await subject.forceRefresh().catch(() => undefined);
    expect(subject.getState().status).toBe("expired");
    expect(storage.value?.refreshToken).toBe("refresh-access-a");
  });

  it("does not expose an expired restored session before refresh succeeds", async () => {
    const storage = new MemoryStorage();
    storage.value = {
      version: 1,
      sessionId: "session-123456789",
      subject: "identity-a",
      issuer: config.oauthIssuer,
      clientId: config.oauthClientId,
      accessToken: "expired-access",
      refreshToken: "refresh-expired-access",
      expiresAt: 1,
      scope: "openid offline_access",
    };
    const transport = new FakeTransport();
    const subject = manager(storage, transport, () => 60_000);
    const observed: string[] = [];
    subject.subscribe((state) => observed.push(state.status));

    await subject.restore();

    expect(observed).toEqual(["loading", "signedIn"]);
    expect(subject.getState().status).toBe("signedIn");
    expect(storage.value?.accessToken).toBe("access-b");
  });

  it("clears the prior identity boundary and rotates before restoring", async () => {
    const storage = new MemoryStorage();
    storage.value = {
      version: 1,
      sessionId: "session-123456789",
      subject: "identity-a",
      issuer: config.oauthIssuer,
      clientId: config.oauthClientId,
      accessToken: "access-a",
      refreshToken: "refresh-access-a",
      expiresAt: 120_000,
      scope: "openid offline_access",
    };
    const transport = new FakeTransport();
    const calls: string[] = [];
    const subject = manager(
      storage,
      transport,
      () => 0,
      async () => {
        calls.push("boundary-cleared");
      },
    );

    await subject.restore();

    expect(calls).toEqual(["boundary-cleared"]);
    expect(subject.getState().status).toBe("signedIn");
    expect(transport.refreshCalls).toBe(1);
    expect(storage.value?.accessToken).toBe("access-b");
  });

  it("clears local state even when remote logout is offline", async () => {
    const storage = new MemoryStorage();
    const transport = new FakeTransport();
    const subject = manager(storage, transport, () => 0);
    await subject.signIn();
    transport.revokeError = new AuthFailure("offline");
    await subject.signOut();
    expect(transport.revokeCalls).toBe(1);
    expect(subject.getState().status).toBe("signedOut");
    expect(storage.value).toBe(null);
  });

  it("hides protected state before a slow remote revocation finishes", async () => {
    const storage = new MemoryStorage();
    const transport = new FakeTransport();
    const subject = manager(storage, transport, () => 0);
    await subject.signIn();
    let release = () => {};
    transport.revokeGate = new Promise<void>((resolve) => {
      release = resolve;
    });

    const pending = subject.signOut();
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();

    expect(subject.getState().status).toBe("signedOut");
    expect(storage.value).toBe(null);
    release();
    await pending;
  });

  it("does not resurrect the session when a successful refresh lands after sign-out", async () => {
    const storage = new MemoryStorage();
    const transport = new FakeTransport();
    const subject = manager(storage, transport, () => 0);
    await subject.signIn();

    // Park the refresh at the token endpoint, then start it.
    let releaseRefresh = () => {};
    transport.refreshGate = new Promise<void>((resolve) => {
      releaseRefresh = resolve;
    });
    const pending = subject.forceRefresh().catch(() => undefined);
    await Promise.resolve();
    expect(transport.refreshCalls).toBe(1);

    // Sign out while the refresh is in flight.
    await subject.signOut();
    expect(subject.getState().status).toBe("signedOut");
    expect(storage.value).toBe(null);

    // The token endpoint now succeeds — but the session it belonged to is gone,
    // so it must NOT save tokens, restore state, or publish signedIn.
    releaseRefresh();
    await pending;

    expect(subject.getState().status).toBe("signedOut");
    expect(storage.value).toBe(null);
  });

  it("does not republish signed-in/expired state when a stale refresh fails after sign-out", async () => {
    const storage = new MemoryStorage();
    const transport = new FakeTransport();
    const subject = manager(storage, transport, () => 0);
    await subject.signIn();

    let releaseRefresh = () => {};
    transport.refreshGate = new Promise<void>((resolve) => {
      releaseRefresh = resolve;
    });
    // An "offline" failure would normally publish `expired` — after sign-out it
    // must not overwrite the signed-out state.
    transport.refreshError = new AuthFailure("offline");
    const pending = subject.forceRefresh().catch(() => undefined);
    await Promise.resolve();
    expect(transport.refreshCalls).toBe(1);

    await subject.signOut();
    expect(subject.getState().status).toBe("signedOut");
    expect(storage.value).toBe(null);

    releaseRefresh();
    await pending;

    expect(subject.getState().status).toBe("signedOut");
    expect(storage.value).toBe(null);
  });

  it("starts feature cleanup with the current token without delaying local logout", async () => {
    const storage = new MemoryStorage();
    const transport = new FakeTransport();
    const subject = manager(storage, transport, () => 0);
    await subject.signIn();
    let release = () => {};
    const cleanupGate = new Promise<void>((resolve) => {
      release = resolve;
    });
    let cleanupToken: string | null = null;
    subject.registerSessionClearHook(async (session) => {
      cleanupToken = session?.accessToken ?? null;
      await cleanupGate;
    });

    const pending = subject.signOut();
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
    expect(String(cleanupToken)).toBe("access-a");
    expect(subject.getState().status).toBe("signedOut");
    expect(storage.value).toBe(null);
    release();
    await pending;
  });
});

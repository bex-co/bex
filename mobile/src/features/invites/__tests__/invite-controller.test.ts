import {
  InviteFlowController,
  classifyInviteAcceptanceError,
  type AcceptedWorkspace,
  type InviteAcceptanceClient,
} from "../invite-controller";
import type { InviteStore } from "../invite-storage";
import type { StoredInvite } from "../invite-token";

const token = "0123456789abcdef0123456789abcdef";
const accepted: AcceptedWorkspace = {
  id: "tea-workspace1",
  name: "Acme",
  role: "DEVELOPER",
};

class MemoryStore implements InviteStore {
  value: StoredInvite | null = null;
  saves = 0;
  clears = 0;
  saveError?: Error;
  clearError?: Error;
  async load() {
    return this.value;
  }
  async save(value: StoredInvite) {
    this.saves += 1;
    if (this.saveError) throw this.saveError;
    this.value = value;
  }
  async clear() {
    this.clears += 1;
    this.value = null;
    if (this.clearError) throw this.clearError;
  }
}

class Client implements InviteAcceptanceClient {
  calls = 0;
  tokens: string[] = [];
  result: Promise<AcceptedWorkspace> = Promise.resolve(accepted);
  accept(received: string) {
    this.calls += 1;
    this.tokens.push(received);
    return this.result;
  }
}

function setup(timeoutMs = 15_000) {
  const store = new MemoryStore();
  const client = new Client();
  let refreshes = 0;
  let refreshError: Error | null = null;
  const states: string[] = [];
  const controller = new InviteFlowController(
    store,
    client,
    async () => {
      refreshes += 1;
      if (refreshError) throw refreshError;
    },
    (state) => states.push(state.status),
    timeoutMs,
  );
  return {
    controller,
    store,
    client,
    states,
    refreshes: () => refreshes,
    failRefresh: () => {
      refreshError = new Error("offline");
    },
  };
}

describe("native invite flow", () => {
  it("stashes before OAuth and binds exactly the first signed-in subject", async () => {
    const { controller, store, client } = setup();
    await controller.bootstrap(null);
    expect(await controller.capture(token, null)).toBe(true);
    expect(store.value?.subject).toBe(null);
    expect(client.calls).toBe(0);

    await controller.syncSubject("identity-a");
    expect(store.value?.subject).toBe("identity-a");
    expect(controller.getState().status).toBe("ready");
    expect(JSON.stringify(controller.getState()).includes(token)).toBe(false);
  });

  it("clears a bearer instead of carrying it across subjects", async () => {
    const { controller, store } = setup();
    store.value = { version: 1, token, subject: "identity-a" };
    await controller.bootstrap("identity-b");
    expect(store.value === null).toBe(true);
    expect(controller.getState()).toEqual({
      status: "terminal",
      failure: "subject-changed",
    });
  });

  it("requires an explicit accept, single-flights it, clears, and refreshes", async () => {
    const { controller, store, client, refreshes } = setup();
    await controller.bootstrap("identity-a");
    await controller.capture(token, "identity-a");
    let resolve = (_value: AcceptedWorkspace) => {};
    client.result = new Promise<AcceptedWorkspace>((done) => {
      resolve = done;
    });

    const first = controller.accept("identity-a");
    const second = controller.accept("identity-a");
    await Promise.resolve();
    await Promise.resolve();
    expect(client.calls).toBe(1);
    expect(controller.getState().status).toBe("accepting");
    resolve(accepted);
    await Promise.all([first, second]);

    expect(client.calls).toBe(1);
    expect(client.tokens).toEqual([token]);
    expect(store.value).toBe(null);
    expect(refreshes()).toBe(1);
    expect(controller.getState()).toEqual({
      status: "accepted",
      workspace: accepted,
      refreshFailed: false,
    });
  });

  it("clears every stable terminal server outcome", async () => {
    const cases = [
      ["INVITE_INVALID", "invalid"],
      ["INVITE_EXPIRED", "expired"],
      ["INVITE_ALREADY_ACCEPTED", "already-accepted"],
      ["INVITE_PLAN_LIMIT", "plan-limit"],
      ["FORBIDDEN", "authorization"],
      ["SOMETHING_ELSE", "failed"],
    ] as const;
    for (const [code, failure] of cases) {
      const { controller, store, client } = setup();
      await controller.bootstrap("identity-a");
      await controller.capture(token, "identity-a");
      client.result = Promise.reject({
        errors: [{ extensions: { code } }],
      });
      await controller.accept("identity-a");
      expect(store.value).toBe(null);
      expect(controller.getState()).toEqual({ status: "terminal", failure });
    }
  });

  it("retains only transport and unavailable failures for explicit retry", async () => {
    for (const error of [
      Object.assign(new Error("timed out"), { name: "TimeoutError" }),
      { networkError: { statusCode: 503 } },
      new TypeError("network request failed"),
    ]) {
      const { controller, store, client } = setup();
      await controller.bootstrap("identity-a");
      await controller.capture(token, "identity-a");
      client.result = Promise.reject(error);
      await controller.accept("identity-a");
      expect(store.value?.token).toBe(token);
      expect(controller.getState().status).toBe("retryable");
      expect(client.calls).toBe(1);
    }
  });

  it("bounds a hung request without automatic replay", async () => {
    const { controller, store, client } = setup(5);
    await controller.bootstrap("identity-a");
    await controller.capture(token, "identity-a");
    client.result = new Promise(() => {});
    await controller.accept("identity-a");
    expect(client.calls).toBe(1);
    expect(store.value?.token).toBe(token);
    expect(controller.getState()).toEqual({
      status: "retryable",
      failure: "transport",
    });
  });

  it("keeps success terminal even if workspace refresh fails", async () => {
    const { controller, store, failRefresh } = setup();
    failRefresh();
    await controller.bootstrap("identity-a");
    await controller.capture(token, "identity-a");
    await controller.accept("identity-a");
    expect(store.value).toBe(null);
    expect(controller.getState()).toEqual({
      status: "accepted",
      workspace: accepted,
      refreshFailed: true,
    });
  });

  it("clears synchronously in memory on logout and aborts late publication", async () => {
    const { controller, store, client } = setup();
    await controller.bootstrap("identity-a");
    await controller.capture(token, "identity-a");
    let resolve = (_value: AcceptedWorkspace) => {};
    client.result = new Promise<AcceptedWorkspace>((done) => {
      resolve = done;
    });
    const accepting = controller.accept("identity-a");
    await Promise.resolve();
    const clearing = controller.clear();
    expect(controller.getState().status).toBe("empty");
    resolve(accepted);
    await Promise.all([accepting, clearing]);
    expect(store.value).toBe(null);
    expect(controller.getState().status).toBe("empty");
  });
});

describe("invite error classifier", () => {
  it("reads exact nested GraphQL codes without message matching", () => {
    expect(
      classifyInviteAcceptanceError({
        graphQLErrors: [{ extensions: { code: "INVITE_ALREADY_ACCEPTED" } }],
      }),
    ).toBe("already-accepted");
    // A raw 404 is a GraphQL transport/proxy failure. Only the server's stable
    // INVITE_INVALID extension may terminally clear a bearer.
    expect(classifyInviteAcceptanceError({ statusCode: 404 })).toBe(
      "unavailable",
    );
    expect(classifyInviteAcceptanceError({ status: 429 })).toBe("unavailable");
  });
});

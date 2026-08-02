import { CurrentUserController } from "../current-user-controller";
import {
  CurrentUserError,
  type CurrentUserClient,
} from "../current-user-client";
import type { CurrentUser } from "../current-user";

type Deferred = {
  promise: Promise<CurrentUser>;
  resolve: (user: CurrentUser) => void;
  reject: (error: unknown) => void;
};

function deferred(): Deferred {
  let resolve!: (user: CurrentUser) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<CurrentUser>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

/** A client whose every fetch is a controllable deferred, so the test drives
 * resolution order to exercise the identity-boundary suppression. */
function fakeClient() {
  const pending: Deferred[] = [];
  const client = {
    fetch: () => {
      const d = deferred();
      pending.push(d);
      return d.promise;
    },
  } as unknown as CurrentUserClient;
  return { client, pending };
}

function track(controller: CurrentUserController) {
  const states: string[] = [];
  controller.subscribe((state) => states.push(state.status));
  return states;
}

describe("CurrentUserController", () => {
  it("moves loading → ready with the fetched user", async () => {
    const { client, pending } = fakeClient();
    const controller = new CurrentUserController(client);
    const load = controller.load();
    pending[0].resolve({ name: "Test Person", email: "p@example.test" });
    await load;
    const state = controller.getState();
    expect(state.status).toBe("ready");
    expect(state.status === "ready" && state.user.name).toBe("Test Person");
  });

  it("maps a network error to offline and a server error to unavailable", async () => {
    const offline = fakeClient();
    const c1 = new CurrentUserController(offline.client);
    const l1 = c1.load();
    offline.pending[0].reject(new CurrentUserError("network", "down"));
    await l1;
    expect(c1.getState().status).toBe("offline");

    const bad = fakeClient();
    const c2 = new CurrentUserController(bad.client);
    const l2 = c2.load();
    bad.pending[0].reject(new CurrentUserError("unavailable", "503"));
    await l2;
    expect(c2.getState().status).toBe("unavailable");
  });

  it("suppresses a superseded response so the latest identity wins", async () => {
    const { client, pending } = fakeClient();
    const controller = new CurrentUserController(client);
    const first = controller.load();
    const second = controller.load();
    // The newer request resolves first, then the stale one resolves late.
    pending[1].resolve({ name: "Current User", email: "cur@example.test" });
    pending[0].resolve({ name: "Stale User", email: "old@example.test" });
    await Promise.all([first, second]);
    const state = controller.getState();
    expect(state.status === "ready" && state.user.name).toBe("Current User");
  });

  it("reset aborts the in-flight read and returns to idle for a late reply", async () => {
    const { client, pending } = fakeClient();
    const controller = new CurrentUserController(client);
    const states = track(controller);
    const load = controller.load();
    controller.reset();
    // A late reply from the now-abandoned session must not render.
    pending[0].resolve({ name: "Signed Out", email: "gone@example.test" });
    await load;
    expect(controller.getState().status).toBe("idle");
    expect(states).toEqual(["idle", "loading", "idle"]);
  });
});

import { NotificationInboxStore, type KeyValueStorage } from "../inbox-store";
import type { NotificationEnvelope } from "../deep-link";

class MemoryStorage implements KeyValueStorage {
  values = new Map<string, string>();
  async getItem(key: string) {
    return this.values.get(key) ?? null;
  }
  async setItem(key: string, value: string) {
    this.values.set(key, value);
  }
  async removeItem(key: string) {
    this.values.delete(key);
  }
}

const envelope: NotificationEnvelope = {
  schema: "bex.notification.v1",
  notificationId: "notification-1",
  subject: "identity-1",
  workspaceId: "tea-a",
  sessionId: "session-1",
  event: "server_failed",
  route: "/services/srv-abc",
};

describe("NotificationInboxStore", () => {
  it("deduplicates receipts and preserves read acknowledgments", async () => {
    const storage = new MemoryStorage();
    const store = new NotificationInboxStore(storage);
    await store.record(envelope, {
      title: "First",
      body: "body",
      receivedAt: 1,
    });
    await store.record(envelope, {
      title: "Updated",
      body: "body",
      receivedAt: 2,
    });
    expect((await store.list()).map((item) => item.title)).toEqual(["Updated"]);
    await store.markRead(envelope.notificationId);
    expect((await store.list())[0]?.read).toBe(true);
  });

  it("clears local content at logout", async () => {
    const storage = new MemoryStorage();
    const store = new NotificationInboxStore(storage);
    await store.record(envelope, {
      title: "Failure",
      body: "body",
      receivedAt: 1,
    });
    await store.clear();
    expect(await store.list()).toEqual([]);
  });

  it("serializes an in-flight receipt before logout clearing", async () => {
    const storage = new MemoryStorage();
    const store = new NotificationInboxStore(storage);
    const recording = store.record(envelope, {
      title: "Failure",
      body: "body",
      receivedAt: 1,
    });
    const clearing = store.clear();
    await Promise.all([recording, clearing]);
    expect(await store.list()).toEqual([]);
  });

  it("merges the durable server inbox without reviving locally read items", async () => {
    const storage = new MemoryStorage();
    const store = new NotificationInboxStore(storage);
    await store.record(envelope, {
      title: "Local",
      body: "body",
      receivedAt: 1,
    });
    await store.markRead(envelope.notificationId);
    const items = await store.mergeRemote([
      {
        id: envelope.notificationId,
        event: envelope.event,
        route: envelope.route,
        title: "Durable",
        body: "server body",
        receivedAt: 2,
        read: false,
      },
      {
        id: "notification-evil",
        event: "server_failed",
        route: "https://evil.test/services/srv-abc",
        title: "invalid",
        body: "invalid",
        receivedAt: 3,
        read: false,
      },
    ]);
    expect(items.length).toBe(1);
    expect(items[0]?.id).toBe(envelope.notificationId);
    expect(items[0]?.title).toBe("Durable");
    expect(items[0]?.read).toBe(true);
  });

  it("drops tampered routes and malformed timestamps from persisted data", async () => {
    const storage = new MemoryStorage();
    storage.values.set(
      "bex.notifications.inbox.v1",
      JSON.stringify([
        {
          id: "notification-1",
          event: "server_failed",
          route: "https://evil.test/services/srv-abc",
          title: "tampered",
          body: "body",
          receivedAt: 1,
          read: false,
        },
        {
          id: "notification-2",
          event: "server_failed",
          route: "/services/srv-abc",
          title: "malformed",
          body: "body",
          receivedAt: null,
          read: false,
        },
      ]),
    );
    const store = new NotificationInboxStore(storage);
    expect(await store.list()).toEqual([]);
  });
  it("removes confirmed revoked destinations without treating a page as global absence", async () => {
    const storage = new MemoryStorage();
    const store = new NotificationInboxStore(storage);
    await store.record(envelope, { receivedAt: 1 });
    await store.record(
      {
        ...envelope,
        notificationId: "agent-1",
        event: "agent_pr_ready",
        route: "/sessions/ags-one",
      },
      { receivedAt: 2 },
    );
    const next = await store.reconcile(
      (item) => !item.route.startsWith("/sessions/"),
    );
    expect(next.map((item) => item.id)).toEqual([envelope.notificationId]);
    expect(await store.list()).toEqual(next);
    expect(await store.mergeRemote([])).toEqual(next);
  });

  it("rolls back an old-generation write before a replacement store reconciles", async () => {
    const storage = new MemoryStorage();
    let current = true;
    let release: () => void = () => undefined;
    let started: () => void = () => undefined;
    const writing = new Promise<void>((resolve) => {
      started = resolve;
    });
    const delayed = new Promise<void>((resolve) => {
      release = resolve;
    });
    const originalSet = storage.setItem.bind(storage);
    let delayOnce = true;
    storage.setItem = async (key, value) => {
      if (delayOnce) {
        delayOnce = false;
        started();
        await delayed;
      }
      await originalSet(key, value);
    };
    const oldStore = new NotificationInboxStore(
      storage,
      "shared",
      100,
      () => current,
    );
    const recording = oldStore.record(envelope, { receivedAt: 1 });
    await writing;
    current = false;
    const nextStore = new NotificationInboxStore(storage, "shared");
    const reconciling = nextStore.reconcile();
    release();
    await recording;
    expect(await reconciling).toEqual([]);
    expect(await nextStore.list()).toEqual([]);
  });
  it("retains authorized older history when a remote page omits it", async () => {
    const store = new NotificationInboxStore(new MemoryStorage());
    await store.record(envelope, { receivedAt: 1 });
    const next = await store.mergeRemote([
      {
        id: "newest",
        event: "server_failed",
        route: "/services/srv-new",
        title: "New",
        body: "",
        receivedAt: 100,
        read: false,
      },
    ]);
    expect(next.map((item) => item.id)).toEqual([
      "newest",
      envelope.notificationId,
    ]);
  });
});

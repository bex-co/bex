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
  event: "server_failed",
  route: "/services/srv-abc",
};

describe("NotificationInboxStore", () => {
  it("deduplicates receipts and reconciles unread badges", async () => {
    const storage = new MemoryStorage();
    const badges: number[] = [];
    const store = new NotificationInboxStore(storage, {
      set: async (count) => badges.push(count),
    });
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
    expect(badges).toEqual([1, 1]);
    await store.markRead(envelope.notificationId);
    expect(badges.at(-1)).toBe(0);
  });

  it("clears local content and the badge at logout", async () => {
    const storage = new MemoryStorage();
    const badges: number[] = [];
    const store = new NotificationInboxStore(storage, {
      set: async (count) => badges.push(count),
    });
    await store.record(envelope, {
      title: "Failure",
      body: "body",
      receivedAt: 1,
    });
    await store.clear();
    expect(await store.list()).toEqual([]);
    expect(badges.at(-1)).toBe(0);
  });

  it("serializes an in-flight receipt before logout clearing", async () => {
    const storage = new MemoryStorage();
    const store = new NotificationInboxStore(storage, {
      set: async () => undefined,
    });
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
    const badges: number[] = [];
    const store = new NotificationInboxStore(storage, {
      set: async (count) => badges.push(count),
    });
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
    expect(badges.at(-1)).toBe(0);
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
    const store = new NotificationInboxStore(storage, {
      set: async () => undefined,
    });
    expect(await store.list()).toEqual([]);
  });
});

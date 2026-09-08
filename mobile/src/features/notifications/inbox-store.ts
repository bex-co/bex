import {
  parseNotificationEnvelope,
  type NotificationEnvelope,
  type NotificationEvent,
  type NotificationRoute,
} from "./deep-link";

export type NotificationInboxItem = {
  id: string;
  event: NotificationEvent;
  route: NotificationRoute;
  title: string;
  body: string;
  receivedAt: number;
  read: boolean;
};

export type RemoteNotificationInboxItem = {
  id: string;
  event: string;
  route: string;
  title: string;
  body: string;
  receivedAt: number;
  read: boolean;
};

export interface KeyValueStorage {
  getItem(key: string): Promise<string | null>;
  setItem(key: string, value: string): Promise<void>;
  removeItem(key: string): Promise<void>;
}

const isItem = (value: unknown): value is NotificationInboxItem => {
  if (!value || typeof value !== "object") return false;
  const item = value as Partial<NotificationInboxItem>;
  const envelope = parseNotificationEnvelope({
    schema: "bex.notification.v1",
    notificationId: item.id,
    event: item.event,
    route: item.route,
    subject: "stored-subject",
    workspaceId: "stored-workspace",
    sessionId: "stored-session",
  });
  return (
    envelope !== null &&
    typeof item.id === "string" &&
    typeof item.event === "string" &&
    typeof item.route === "string" &&
    typeof item.title === "string" &&
    typeof item.body === "string" &&
    typeof item.receivedAt === "number" &&
    Number.isFinite(item.receivedAt) &&
    typeof item.read === "boolean"
  );
};

// A reset can create a new store for the same persistence key while an old
// write is still pending. Serialize both instances, including logout cleanup.
const storageQueues = new WeakMap<
  KeyValueStorage,
  Map<string, Promise<void>>
>();

export class NotificationInboxStore {
  constructor(
    private readonly storage: KeyValueStorage,
    private readonly key = "bex.notifications.inbox.v1",
    private readonly limit = 100,
    private readonly isCurrent: () => boolean = () => true,
  ) {}

  async list(): Promise<NotificationInboxItem[]> {
    const raw = await this.storage.getItem(this.key).catch(() => null);
    if (!raw) return [];
    try {
      const parsed: unknown = JSON.parse(raw);
      return Array.isArray(parsed)
        ? parsed.filter(isItem).slice(0, this.limit)
        : [];
    } catch {
      return [];
    }
  }

  async record(
    envelope: NotificationEnvelope,
    content: {
      title?: string | null;
      body?: string | null;
      receivedAt: number;
    },
  ): Promise<NotificationInboxItem[]> {
    return this.mutate(async () => {
      const existing = await this.list();
      const previous = existing.find(
        (item) => item.id === envelope.notificationId,
      );
      const next: NotificationInboxItem = {
        id: envelope.notificationId,
        event: envelope.event,
        route: envelope.route,
        title: (content.title ?? "bex").slice(0, 200),
        body: (content.body ?? "").slice(0, 2_000),
        receivedAt: Number.isFinite(content.receivedAt)
          ? content.receivedAt
          : Date.now(),
        read: previous?.read ?? false,
      };
      const items = [next, ...existing.filter((item) => item.id !== next.id)]
        .sort((a, b) => b.receivedAt - a.receivedAt)
        .slice(0, this.limit);
      await this.persist(items);
      return items;
    });
  }

  async mergeRemote(
    remote: RemoteNotificationInboxItem[],
  ): Promise<NotificationInboxItem[]> {
    return this.mutate(async () => {
      const existing = await this.list();
      const byID = new Map(existing.map((item) => [item.id, item]));
      for (const candidate of remote.slice(0, this.limit)) {
        const envelope = parseNotificationEnvelope({
          schema: "bex.notification.v1",
          notificationId: candidate.id,
          event: candidate.event,
          route: candidate.route,
          subject: "stored-subject",
          workspaceId: "stored-workspace",
          sessionId: "stored-session",
        });
        if (
          !envelope ||
          typeof candidate.title !== "string" ||
          typeof candidate.body !== "string" ||
          !Number.isFinite(candidate.receivedAt) ||
          typeof candidate.read !== "boolean"
        ) {
          continue;
        }
        const previous = byID.get(candidate.id);
        byID.set(candidate.id, {
          id: envelope.notificationId,
          event: envelope.event,
          route: envelope.route,
          title: candidate.title.slice(0, 200),
          body: candidate.body.slice(0, 2_000),
          receivedAt: candidate.receivedAt,
          read: candidate.read || previous?.read === true,
        });
      }
      const items = [...byID.values()]
        .sort((a, b) => b.receivedAt - a.receivedAt)
        .slice(0, this.limit);
      await this.persist(items);
      return items;
    });
  }

  async markRead(id: string): Promise<NotificationInboxItem[]> {
    return this.mutate(async () => {
      const items = (await this.list()).map((item) =>
        item.id === id ? { ...item, read: true } : item,
      );
      await this.persist(items);
      return items;
    });
  }

  async markAllRead(): Promise<NotificationInboxItem[]> {
    return this.mutate(async () => {
      const items = (await this.list()).map((item) => ({
        ...item,
        read: true,
      }));
      await this.persist(items);
      return items;
    });
  }

  // Callers supply only confirmed authorization removals. An unavailable
  // capability check or an omitted row on a bounded remote page is not a denial.
  async reconcile(
    retain: (item: NotificationInboxItem) => boolean = () => true,
  ): Promise<NotificationInboxItem[]> {
    return this.mutate(async () => {
      const existing = await this.list();
      const items = existing.filter(retain);
      if (items.length !== existing.length) await this.persist(items);
      return this.isCurrent() ? items : [];
    });
  }

  async clear(): Promise<void> {
    await this.mutate(async () => {
      await this.storage.removeItem(this.key).catch(() => undefined);
    });
  }

  private mutate<T>(operation: () => Promise<T>): Promise<T> {
    let queues = storageQueues.get(this.storage);
    if (!queues) {
      queues = new Map();
      storageQueues.set(this.storage, queues);
    }
    const pending = queues.get(this.key) ?? Promise.resolve();
    const result = pending.then(operation, operation);
    const settled = result.then(
      () => undefined,
      () => undefined,
    );
    queues.set(this.key, settled);
    void settled.then(() => {
      if (queues.get(this.key) === settled) queues.delete(this.key);
    });
    return result;
  }

  private async persist(items: NotificationInboxItem[]): Promise<void> {
    if (!this.isCurrent()) return;
    const previous = await this.storage.getItem(this.key);
    if (!this.isCurrent()) return;
    await this.storage.setItem(this.key, JSON.stringify(items));
    if (!this.isCurrent()) {
      // Roll back a write that crossed the boundary before releasing the key
      // queue. A new generation cannot observe it through a store mutation.
      if (previous === null) await this.storage.removeItem(this.key);
      else await this.storage.setItem(this.key, previous);
      return;
    }
  }
}

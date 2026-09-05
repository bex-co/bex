import { LogSession } from "../log-session";
import {
  LogApiError,
  type LogFilters,
  type LogHistoryPage,
  type LogLine,
  type LogTailCallbacks,
  type LogTransport,
} from "../types";

const boundary = "2026-08-02T10:00:00.000Z";
const historyLine: LogLine = {
  id: "same",
  message: "history",
  timestamp: boundary,
  labels: [],
};

class FakeTransport implements LogTransport {
  historyFilters?: LogFilters;
  subscriptions: LogFilters[] = [];
  callbacks?: LogTailCallbacks;
  closed = 0;
  page: LogHistoryPage = {
    hasMore: false,
    nextStartTime: boundary,
    nextEndTime: boundary,
    logs: [historyLine],
  };

  async history(filters: LogFilters): Promise<LogHistoryPage> {
    this.historyFilters = filters;
    return this.page;
  }

  async subscribe(
    filters: LogFilters,
    callbacks: LogTailCallbacks,
  ): Promise<{ close: () => void }> {
    this.subscriptions.push(filters);
    this.callbacks = callbacks;
    return { close: () => (this.closed += 1) };
  }
}

describe("LogSession", () => {
  it("uses one exact catch-up boundary and deduplicates the inclusive handoff", async () => {
    const transport = new FakeTransport();
    const subject = new LogSession(transport, 5, () => new Date(boundary));
    await subject.start({ resource: "srv-1", types: ["app"] });

    expect(transport.historyFilters?.endTime).toBe(boundary);
    expect(transport.historyFilters?.limit).toBe(100);
    expect(transport.subscriptions[0].startTime).toBe(boundary);
    transport.callbacks?.onLine(historyLine);
    transport.callbacks?.onLine({
      ...historyLine,
      id: "live",
      message: "live",
      timestamp: "2026-08-02T10:00:01.000Z",
    });
    expect(subject.snapshot().lines.map(({ id }) => id)).toEqual([
      "same",
      "live",
    ]);
  });

  it("keeps a store-only filter as explicit history-only state", async () => {
    const transport = new FakeTransport();
    const subject = new LogSession(transport, 5, () => new Date(boundary));
    await subject.start({ resource: "srv-1", statusCode: ["5xx"] });
    expect(subject.snapshot().phase).toBe("history_only");
    expect(subject.snapshot().tailBlockedByStoreOnlyFilters).toBe(true);
    expect(transport.subscriptions.length).toBe(0);
  });

  it("reconnects from the newest retained timestamp through an injectable scheduler", async () => {
    const transport = new FakeTransport();
    let reconnect: (() => void) | undefined;
    let delay = 0;
    const subject = new LogSession(
      transport,
      5,
      () => new Date(boundary),
      (callback, delayMs) => {
        reconnect = callback;
        delay = delayMs;
        return () => {
          reconnect = undefined;
        };
      },
    );
    await subject.start({ resource: "srv-1", types: ["app"] });
    transport.callbacks?.onLine({
      ...historyLine,
      id: "new",
      timestamp: "2026-08-02T10:00:02.000Z",
    });
    transport.callbacks?.onError(new LogApiError("network", "disconnected"));
    expect(subject.snapshot().phase).toBe("reconnecting");
    expect(delay).toBe(1000);
    reconnect?.();
    await Promise.resolve();
    expect(transport.subscriptions.length).toBe(2);
    expect(transport.subscriptions[1].startTime).toBe(
      "2026-08-02T10:00:02.000Z",
    );
  });

  it("cancels a pending reconnect when the screen leaves", async () => {
    const transport = new FakeTransport();
    let reconnect: (() => void) | undefined;
    const subject = new LogSession(
      transport,
      5,
      () => new Date(boundary),
      (callback) => {
        reconnect = callback;
        return () => {
          reconnect = undefined;
        };
      },
    );
    await subject.start({ resource: "srv-1" });
    transport.callbacks?.onClose();
    subject.stop();
    expect(reconnect).toBe(undefined);
    expect(subject.snapshot().phase).toBe("idle");
  });
});

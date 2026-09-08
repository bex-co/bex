import { PendingNotificationResponse } from "../pending-response";

describe("PendingNotificationResponse", () => {
  it("retains a cold tap while restoring and deduplicates the listener delivery", async () => {
    const pending = new PendingNotificationResponse<string>();
    const opened: string[] = [];
    const cleared: string[] = [];
    let ready = false;
    const drain = () =>
      pending.drain(
        () => (ready ? "open" : "wait"),
        async (value) => {
          opened.push(value);
          return true;
        },
        (id) => {
          cleared.push(id);
        },
      );
    pending.capture("response-1", "destination");
    await drain();
    expect(opened).toEqual([]);
    expect(cleared).toEqual([]);
    ready = true;
    await drain();
    pending.capture("response-1", "destination");
    await drain();
    expect(opened).toEqual(["destination"]);
    expect(cleared).toEqual(["response-1"]);
  });

  it("discards terminal refusals without navigating", async () => {
    const pending = new PendingNotificationResponse<string>();
    const cleared: string[] = [];
    let opens = 0;
    pending.capture("wrong-workspace", "private");
    await pending.drain(
      () => "reject",
      async () => {
        opens += 1;
        return true;
      },
      (id) => {
        cleared.push(id);
      },
    );
    expect(opens).toBe(0);
    expect(cleared).toEqual(["wrong-workspace"]);
  });

  it("keeps a response whose opening crossed a boundary for revalidation", async () => {
    const pending = new PendingNotificationResponse<string>();
    const cleared: string[] = [];
    pending.capture("response-1", "destination");
    await pending.drain(
      () => "open",
      async () => false,
      (id) => {
        cleared.push(id);
      },
    );
    expect(cleared).toEqual([]);
    await pending.drain(
      () => "reject",
      async () => true,
      (id) => {
        cleared.push(id);
      },
    );
    expect(cleared).toEqual(["response-1"]);
  });
});

import {
  EnvironmentOperationGuard,
  environmentTimeoutError,
} from "../environment-operation-guard";

describe("EnvironmentOperationGuard", () => {
  it("invalidates every in-flight result across a lifecycle boundary", () => {
    const guard = new EnvironmentOperationGuard();
    const reveal = guard.begin("reveal", 60_000);
    const mutation = guard.begin("mutation", 60_000);
    expect(guard.hasActive("reveal")).toBe(true);
    expect(guard.hasActive("mutation")).toBe(true);
    guard.invalidate();
    expect(reveal.signal.aborted).toBe(true);
    expect(mutation.signal.aborted).toBe(true);
    expect(reveal.status()).toBe("invalidated");
    expect(reveal.isCurrent()).toBe(false);
    expect(guard.hasActive("mutation")).toBe(false);
  });

  it("distinguishes bounded timeout from boundary invalidation", async () => {
    const guard = new EnvironmentOperationGuard();
    const lease = guard.begin("mutation", 1);
    await new Promise((resolve) => setTimeout(resolve, 5));
    expect(lease.signal.aborted).toBe(true);
    expect(lease.status()).toBe("timed-out");
    expect(lease.isCurrent()).toBe(true);
    expect(environmentTimeoutError().name).toBe("TimeoutError");
    lease.finish();
  });
});

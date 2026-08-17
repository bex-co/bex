import { describe, it, expect, vi } from "vitest";
import { prefetchInParallel } from "@/common/lib/prefetch";

describe("prefetchInParallel", () => {
  it("runs every thunk", async () => {
    const a = vi.fn().mockResolvedValue(1);
    const b = vi.fn().mockResolvedValue(2);
    await prefetchInParallel([a, b]);
    expect(a).toHaveBeenCalledTimes(1);
    expect(b).toHaveBeenCalledTimes(1);
  });

  it("starts the thunks in parallel, not sequentially", async () => {
    const order: string[] = [];
    const slow = () =>
      new Promise<void>((resolve) =>
        setTimeout(() => {
          order.push("slow");
          resolve();
        }, 20),
      );
    const fast = () =>
      new Promise<void>((resolve) => {
        order.push("fast-start");
        resolve();
      });
    await prefetchInParallel([slow, fast]);
    // fast started before slow resolved => concurrent, not awaited-in-series.
    expect(order[0]).toBe("fast-start");
    expect(order).toContain("slow");
  });

  it("swallows a rejected prefetch so it never blocks navigation", async () => {
    const boom = () => Promise.reject(new Error("prefetch failed"));
    const ok = vi.fn().mockResolvedValue(undefined);
    // Must resolve (not reject) — a failed prefetch is best-effort.
    await expect(prefetchInParallel([boom, ok])).resolves.toBeUndefined();
    expect(ok).toHaveBeenCalled();
  });
});

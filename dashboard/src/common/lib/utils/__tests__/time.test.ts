import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { sleep } from "../time";

describe("Time Utility", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("sleep", () => {
    it("should resolve after specified milliseconds", async () => {
      const promise = sleep(1000);

      // Fast-forward time
      vi.advanceTimersByTime(1000);

      await expect(promise).resolves.toBeUndefined();
    });

    it("should not resolve before timeout", async () => {
      let resolved = false;
      const promise = sleep(1000).then(() => {
        resolved = true;
      });

      // Advance time but not enough
      vi.advanceTimersByTime(500);

      // Give the promise a chance to resolve
      await Promise.resolve();

      expect(resolved).toBe(false);

      // Complete the remaining time
      vi.advanceTimersByTime(500);
      await promise;

      expect(resolved).toBe(true);
    });

    it("should handle zero milliseconds", async () => {
      const promise = sleep(0);

      vi.advanceTimersByTime(0);

      await expect(promise).resolves.toBeUndefined();
    });

    it("should handle very short delays", async () => {
      const promise = sleep(1);

      vi.advanceTimersByTime(1);

      await expect(promise).resolves.toBeUndefined();
    });

    it("should handle long delays", async () => {
      const promise = sleep(60000); // 1 minute

      vi.advanceTimersByTime(60000);

      await expect(promise).resolves.toBeUndefined();
    });

    it("should allow multiple concurrent sleeps", async () => {
      const sleep1 = sleep(100);
      const sleep2 = sleep(200);
      const sleep3 = sleep(300);

      let resolved1 = false;
      let resolved2 = false;
      let resolved3 = false;

      sleep1.then(() => {
        resolved1 = true;
      });
      sleep2.then(() => {
        resolved2 = true;
      });
      sleep3.then(() => {
        resolved3 = true;
      });

      // After 100ms, only first should resolve
      vi.advanceTimersByTime(100);
      await Promise.resolve();
      expect(resolved1).toBe(true);
      expect(resolved2).toBe(false);
      expect(resolved3).toBe(false);

      // After 200ms total, first two should resolve
      vi.advanceTimersByTime(100);
      await Promise.resolve();
      expect(resolved1).toBe(true);
      expect(resolved2).toBe(true);
      expect(resolved3).toBe(false);

      // After 300ms total, all should resolve
      vi.advanceTimersByTime(100);
      await Promise.resolve();
      expect(resolved1).toBe(true);
      expect(resolved2).toBe(true);
      expect(resolved3).toBe(true);
    });

    it("should work in async/await context", async () => {
      const start = Date.now();

      const promise = (async () => {
        await sleep(1000);
        return Date.now() - start;
      })();

      vi.advanceTimersByTime(1000);

      await expect(promise).resolves.toBeDefined();
    });

    it("should resolve to undefined", async () => {
      const promise = sleep(100);

      vi.advanceTimersByTime(100);

      const result = await promise;
      expect(result).toBeUndefined();
    });

    it("should handle decimal milliseconds", async () => {
      const promise = sleep(100.5);

      vi.advanceTimersByTime(101);

      await expect(promise).resolves.toBeUndefined();
    });

    it("should be chainable with then", async () => {
      let executed = false;

      const promise = sleep(100).then(() => {
        executed = true;
        return "done";
      });

      vi.advanceTimersByTime(100);

      const result = await promise;
      expect(executed).toBe(true);
      expect(result).toBe("done");
    });

    it("should work with Promise.all", async () => {
      const promises = [sleep(100), sleep(200), sleep(150)];

      const allPromise = Promise.all(promises);

      // Advance to the longest timeout
      vi.advanceTimersByTime(200);

      await expect(allPromise).resolves.toEqual([
        undefined,
        undefined,
        undefined,
      ]);
    });

    it("should work with Promise.race", async () => {
      const promises = [sleep(300), sleep(100), sleep(200)];

      const racePromise = Promise.race(promises);

      // Advance to the shortest timeout
      vi.advanceTimersByTime(100);

      await expect(racePromise).resolves.toBeUndefined();
    });
  });
});

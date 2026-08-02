import {
  backoffDelayMs,
  RecoveryCanceledError,
  recoverySleep,
} from "../backoff";

describe("recovery backoff", () => {
  it("grows exponentially and remains bounded across server restarts", () => {
    const policy = {
      initialDelayMs: 100,
      maxDelayMs: 500,
      multiplier: 2,
      jitterRatio: 0,
    };
    expect(
      [0, 1, 2, 3, 20].map((retry) => backoffDelayMs(retry, policy)),
    ).toEqual([100, 200, 400, 500, 500]);
  });

  it("injects deterministic symmetric jitter without escaping the cap", () => {
    const policy = {
      initialDelayMs: 100,
      maxDelayMs: 150,
      multiplier: 2,
      jitterRatio: 0.5,
    };
    expect(backoffDelayMs(0, policy, () => 0)).toBe(50);
    expect(backoffDelayMs(0, policy, () => 1)).toBe(150);
    expect(backoffDelayMs(4, policy, () => 1)).toBe(150);
  });

  it("cancels the production timer through AbortSignal", async () => {
    const abort = new AbortController();
    const pending = recoverySleep(60_000, abort.signal);
    abort.abort();
    let canceled = false;
    try {
      await pending;
    } catch (error) {
      canceled = error instanceof RecoveryCanceledError;
    }
    expect(canceled).toBe(true);
  });
});

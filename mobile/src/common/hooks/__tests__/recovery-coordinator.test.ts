import { RecoveryCanceledError, type RecoverySleep } from "../backoff";
import {
  RecoveryCoordinator,
  recoveryAvailable,
  recoveryReasonForTransition,
  type RecoveryEnvironment,
} from "../recovery-coordinator";

const activeOnline: RecoveryEnvironment = {
  connectivity: "online",
  appState: "active",
};
const activeOffline: RecoveryEnvironment = {
  connectivity: "offline",
  appState: "active",
};
const backgroundOnline: RecoveryEnvironment = {
  connectivity: "online",
  appState: "background",
};

const immediateSleep: RecoverySleep = async () => {};

async function microtasks() {
  await Promise.resolve();
  await Promise.resolve();
}

describe("RecoveryCoordinator", () => {
  it("gates recovery while offline and performs one recovery on reconnect", async () => {
    let attempts = 0;
    const coordinator = new RecoveryCoordinator({
      initialEnvironment: activeOffline,
      attempt: async () => {
        attempts += 1;
      },
    });

    expect((await coordinator.request("poll")).status).toBe("deferred");
    expect(attempts).toBe(0);
    coordinator.setEnvironment(activeOnline);
    coordinator.setEnvironment(activeOnline);
    const recovered = await coordinator.request("poll");

    expect(recovered.status).toBe("succeeded");
    expect(attempts).toBe(1);
    expect(coordinator.getSnapshot().lastSucceededAt === null).toBe(false);
  });

  it("aborts background work then recovers once on foreground", async () => {
    let attempts = 0;
    const coordinator = new RecoveryCoordinator({
      initialEnvironment: activeOnline,
      attempt: async ({ signal }) => {
        attempts += 1;
        if (attempts > 1) return;
        await new Promise<void>((_resolve, reject) => {
          signal.addEventListener(
            "abort",
            () => reject(new RecoveryCanceledError()),
            { once: true },
          );
        });
      },
    });

    const first = coordinator.request("stream");
    coordinator.setEnvironment(backgroundOnline);
    expect((await first).status).toBe("deferred");
    expect(coordinator.getSnapshot().phase).toBe("waiting");

    coordinator.setEnvironment(activeOnline);
    coordinator.setEnvironment(activeOnline);
    const second = await coordinator.request("stream");
    expect(second.status).toBe("succeeded");
    expect(attempts).toBe(2);
  });

  it("refreshes auth once and immediately retries the interrupted operation", async () => {
    const calls: string[] = [];
    const unauthorized = new Error("unauthorized");
    const coordinator = new RecoveryCoordinator({
      initialEnvironment: activeOnline,
      attempt: async ({ attempt }) => {
        calls.push(`attempt-${attempt}`);
        if (attempt === 1) throw unauthorized;
      },
      isAuthError: (error) => error === unauthorized,
      refreshAuth: async () => {
        calls.push("refresh-auth");
      },
      sleep: async (delay) => {
        calls.push(`sleep-${delay}`);
      },
    });

    const result = await coordinator.request("poll");
    expect(result).toEqual({ status: "succeeded", attempts: 2 });
    expect(calls).toEqual(["attempt-1", "refresh-auth", "attempt-2"]);
  });

  it("does not storm the auth issuer when refresh itself fails", async () => {
    let refreshes = 0;
    const coordinator = new RecoveryCoordinator({
      initialEnvironment: activeOnline,
      attempt: async () => {
        throw new Error("unauthorized");
      },
      isAuthError: () => true,
      refreshAuth: async () => {
        refreshes += 1;
        throw new Error("issuer restarting");
      },
      maxAttempts: 3,
      sleep: immediateSleep,
    });

    const result = await coordinator.request("poll");
    expect(result.status).toBe("failed");
    expect(refreshes).toBe(1);
  });

  it("uses bounded server-restart backoff before a successful reconnect", async () => {
    const delays: number[] = [];
    let attempts = 0;
    const coordinator = new RecoveryCoordinator({
      initialEnvironment: activeOnline,
      attempt: async () => {
        attempts += 1;
        if (attempts < 4) throw new Error("server restarting");
      },
      maxAttempts: 4,
      backoff: {
        initialDelayMs: 100,
        maxDelayMs: 250,
        multiplier: 2,
        jitterRatio: 0,
      },
      sleep: async (delay) => {
        delays.push(delay);
      },
    });

    const result = await coordinator.request("stream");
    expect(result).toEqual({ status: "succeeded", attempts: 4 });
    expect(delays).toEqual([100, 200, 250]);
  });

  it("shares one in-flight recovery across concurrent callers without a storm", async () => {
    let release = () => {};
    let attempts = 0;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const coordinator = new RecoveryCoordinator({
      initialEnvironment: activeOnline,
      attempt: async () => {
        attempts += 1;
        await gate;
      },
    });

    const first = coordinator.request("poll");
    const second = coordinator.request("stream");
    const third = coordinator.manualRetry();
    expect(first === second).toBe(true);
    expect(second === third).toBe(true);
    expect(attempts).toBe(1);
    release();
    await Promise.all([first, second, third]);
    await microtasks();
    expect(attempts).toBe(1);
  });

  it("cancels a scheduled retry and performs no later attempt", async () => {
    let attempts = 0;
    let sleepStarted = () => {};
    const sleeping = new Promise<void>((resolve) => {
      sleepStarted = resolve;
    });
    const sleep: RecoverySleep = async (_delay, signal) => {
      sleepStarted();
      await new Promise<void>((_resolve, reject) => {
        signal.addEventListener(
          "abort",
          () => reject(new RecoveryCanceledError()),
          { once: true },
        );
      });
    };
    const coordinator = new RecoveryCoordinator({
      initialEnvironment: activeOnline,
      attempt: async () => {
        attempts += 1;
        throw new Error("transport down");
      },
      sleep,
    });

    const pending = coordinator.request("stream");
    await sleeping;
    coordinator.cancel();
    expect((await pending).status).toBe("canceled");
    await microtasks();
    expect(attempts).toBe(1);
    expect(coordinator.getSnapshot().phase).toBe("idle");
  });

  it("exposes pure availability and transition helpers", () => {
    expect(recoveryAvailable(activeOnline)).toBe(true);
    expect(recoveryAvailable(activeOffline)).toBe(false);
    expect(recoveryAvailable(backgroundOnline)).toBe(false);
    expect(recoveryReasonForTransition(activeOffline, activeOnline)).toBe(
      "connectivity",
    );
    expect(recoveryReasonForTransition(backgroundOnline, activeOnline)).toBe(
      "foreground",
    );
    expect(recoveryReasonForTransition(activeOnline, activeOnline)).toBe(null);
  });

  it("stops immediately on classified fatal errors", async () => {
    let attempts = 0;
    const coordinator = new RecoveryCoordinator({
      initialEnvironment: activeOnline,
      attempt: async () => {
        attempts += 1;
        throw new Error("invalid query");
      },
      isRetryable: () => false,
      sleep: immediateSleep,
    });
    const result = await coordinator.request("poll");
    expect(result.status).toBe("failed");
    expect(attempts).toBe(1);
  });
});

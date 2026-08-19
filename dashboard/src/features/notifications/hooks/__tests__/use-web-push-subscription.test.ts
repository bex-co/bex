import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";

const mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate],
}));

import { useWebPushSubscription } from "@/features/notifications/hooks/use-web-push-subscription";

const vapid =
  "BNpgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";

function mockNotification(permission: NotificationPermission) {
  vi.stubGlobal("Notification", {
    permission,
    requestPermission: vi
      .fn()
      .mockResolvedValue(permission === "denied" ? "denied" : "granted"),
  });
}

function installPushEnv(opts: {
  permission: NotificationPermission;
  supported?: boolean;
  subscription?: {
    endpoint: string;
    keys: { p256dh: string; auth: string };
  } | null;
}) {
  const supported = opts.supported ?? true;
  const subscription = opts.subscription
    ? {
        unsubscribe: vi.fn().mockResolvedValue(true),
        toJSON: () => opts.subscription,
      }
    : null;
  const subscribe = vi.fn().mockResolvedValue({
    toJSON: () => ({
      endpoint: "https://push.example/ep",
      keys: { p256dh: "p256", auth: "auth" },
    }),
  });
  const pushManager = {
    getSubscription: vi.fn().mockResolvedValue(subscription),
    subscribe,
  };
  const registration = { pushManager };
  if (supported) {
    Object.defineProperty(navigator, "serviceWorker", {
      configurable: true,
      value: {
        getRegistration: vi.fn().mockResolvedValue(registration),
        register: vi.fn().mockResolvedValue(registration),
      },
    });
    vi.stubGlobal("PushManager", function PushManager() {});
    mockNotification(opts.permission);
  } else {
    vi.unstubAllGlobals();
    Reflect.deleteProperty(window, "PushManager");
  }
  return { subscribe, pushManager };
}

beforeEach(() => {
  mutate.mockReset();
  mutate.mockResolvedValue({});
  window.localStorage.clear();
  vi.unstubAllGlobals();
});

describe("useWebPushSubscription", () => {
  it("reports unsupported when PushManager is missing", async () => {
    installPushEnv({ permission: "default", supported: false });
    const { result } = renderHook(() => useWebPushSubscription(vapid, true));
    await waitFor(() => expect(result.current.status).toBe("unsupported"));
  });

  it("reports unconfigured when the server has no VAPID key", async () => {
    installPushEnv({ permission: "default" });
    const { result } = renderHook(() => useWebPushSubscription("", true));
    await waitFor(() => expect(result.current.status).toBe("unconfigured"));
  });

  it("reports denied when the permission is already denied", async () => {
    installPushEnv({ permission: "denied" });
    const { result } = renderHook(() => useWebPushSubscription(vapid, true));
    await waitFor(() => expect(result.current.status).toBe("denied"));
  });

  it("registers a subscription with a wp- browser id", async () => {
    installPushEnv({ permission: "default" });
    const { result } = renderHook(() => useWebPushSubscription(vapid, true));
    await waitFor(() => expect(result.current.status).toBe("prompt"));
    await act(async () => {
      await result.current.subscribe();
    });
    expect(mutate).toHaveBeenCalled();
    const vars = mutate.mock.calls[0][0].variables as { browserId: string };
    expect(vars.browserId).toMatch(/^wp-/);
    expect(result.current.status).toBe("subscribed");
  });

  it("unsubscribes the browser and the server row", async () => {
    installPushEnv({
      permission: "granted",
      subscription: {
        endpoint: "https://push.example/ep",
        keys: { p256dh: "p256", auth: "auth" },
      },
    });
    const { result } = renderHook(() => useWebPushSubscription(vapid, true));
    await waitFor(() => expect(result.current.status).toBe("subscribed"));
    await act(async () => {
      await result.current.unsubscribe();
    });
    expect(mutate).toHaveBeenCalled();
    expect(result.current.status).toBe("prompt");
  });
});

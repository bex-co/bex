import {
  NotificationRegistrationController,
  type DeviceSubscriptionClient,
  type NotificationNativeAdapter,
  type PermissionState,
} from "../registration-controller";

class Native implements NotificationNativeAdapter {
  current: PermissionState = "undetermined";
  requested: PermissionState = "granted";
  calls: string[] = [];
  async permission() {
    this.calls.push("permission");
    return this.current;
  }
  async requestPermission() {
    this.calls.push("request");
    return this.requested;
  }
  async ensureAndroidChannel() {
    this.calls.push("channel");
  }
  async expoToken(projectId: string) {
    this.calls.push(`token:${projectId}`);
    return "ExponentPushToken[token]";
  }
}

class Subscriptions implements DeviceSubscriptionClient {
  calls: string[] = [];
  available = true;
  unregisterError?: Error;
  async list() {
    this.calls.push("list");
    return { available: this.available };
  }
  async register(input: { deviceId: string }) {
    this.calls.push(`register:${input.deviceId}`);
  }
  async unregister(deviceId: string) {
    this.calls.push(`unregister:${deviceId}`);
    if (this.unregisterError) throw this.unregisterError;
  }
}

function setup(projectId: string | null = "project-1") {
  const native = new Native();
  const subscriptions = new Subscriptions();
  let enabled = false;
  const controller = new NotificationRegistrationController(
    projectId,
    "ios",
    native,
    subscriptions,
    async () => "11111111-1111-4111-8111-111111111111",
    {
      wasEnabled: async () => enabled,
      setEnabled: async (value) => {
        enabled = value;
      },
    },
    () => true,
    () => true,
  );
  return {
    controller,
    native,
    subscriptions,
    setEnabled: (value: boolean) => {
      enabled = value;
    },
    isEnabled: () => enabled,
  };
}

describe("NotificationRegistrationController", () => {
  it("never requests permission during passive inspection", async () => {
    const { controller, native } = setup();
    await controller.inspectAndRepair();
    expect(controller.state).toBe("disabled");
    expect(native.calls).toEqual(["permission"]);
  });

  it("does not register from ambient OS permission before the in-app gesture", async () => {
    const { controller, native, subscriptions } = setup();
    native.current = "granted";
    await controller.inspectAndRepair();
    expect(controller.state).toBe("disabled");
    expect(subscriptions.calls).toEqual(["list"]);
  });

  it("requests from the user action and creates the channel before the token", async () => {
    const { controller, native, subscriptions } = setup();
    await controller.enableFromUserGesture();
    expect(controller.state).toBe("enabled");
    expect(native.calls).toEqual([
      "channel",
      "request",
      "channel",
      "token:project-1",
    ]);
    expect(subscriptions.calls).toEqual([
      "list",
      "register:11111111-1111-4111-8111-111111111111",
    ]);
  });

  it("reports denied, revoked, and unconfigured honestly", async () => {
    const denied = setup();
    denied.native.requested = "denied";
    await denied.controller.enableFromUserGesture();
    expect(denied.controller.state).toBe("denied");
    denied.native.current = "denied";
    denied.setEnabled(true);
    await denied.controller.inspectAndRepair();
    expect(denied.controller.state).toBe("revoked");
    const unconfigured = setup(null);
    await unconfigured.controller.enableFromUserGesture();
    expect(unconfigured.controller.state).toBe("unconfigured");
    expect(unconfigured.native.calls).toEqual([]);
  });

  it("stops before OS permission and token APIs when server push is unavailable", async () => {
    const inspected = setup();
    inspected.subscriptions.available = false;
    await inspected.controller.inspectAndRepair();
    expect(inspected.controller.state).toBe("unavailable");
    expect(inspected.native.calls).toEqual([]);
    expect(inspected.subscriptions.calls).toEqual(["list"]);

    const enabled = setup();
    enabled.subscriptions.available = false;
    await enabled.controller.enableFromUserGesture();
    expect(enabled.controller.state).toBe("unavailable");
    expect(enabled.native.calls).toEqual([]);
    expect(enabled.subscriptions.calls).toEqual(["list"]);
  });

  it("repairs a rotated token best-effort while signed in and online", async () => {
    const { controller, native, subscriptions } = setup();
    native.current = "granted";
    await controller.repairAfterTokenRotation();
    expect(subscriptions.calls).toEqual([
      "list",
      "register:11111111-1111-4111-8111-111111111111",
    ]);
  });

  it("disables locally before a best-effort remote unregister fails", async () => {
    const { controller, subscriptions, setEnabled, isEnabled } = setup();
    setEnabled(true);
    subscriptions.unregisterError = new Error("offline");
    let failed = false;
    try {
      await controller.unregisterCurrent();
    } catch {
      failed = true;
    }
    expect(failed).toBe(true);
    expect(controller.state).toBe("disabled");
    expect(isEnabled()).toBe(false);
  });
});

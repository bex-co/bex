export type NotificationRegistrationState =
  | "checking"
  | "unconfigured"
  | "unavailable"
  | "disabled"
  | "denied"
  | "revoked"
  | "enabling"
  | "enabled"
  | "offline"
  | "error";

export type PermissionState = "granted" | "denied" | "undetermined";

export interface NotificationNativeAdapter {
  permission(): Promise<PermissionState>;
  requestPermission(): Promise<PermissionState>;
  ensureAndroidChannel(): Promise<void>;
  expoToken(projectId: string): Promise<string>;
}

export interface DeviceSubscriptionClient {
  /** Reports whether the server has push delivery configured (availability
   * gate). The registered-device list is not consumed by the client. */
  list(): Promise<{ available: boolean }>;
  register(input: {
    deviceId: string;
    sessionId: string;
    provider: "expo";
    platform: "ios" | "android";
    token: string;
  }): Promise<void>;
  unregister(deviceId: string, accessToken?: string): Promise<void>;
}

export type NotificationBinding = {
  subject: string;
  sessionId: string;
  workspaceId: string;
};

export interface RegistrationPreference {
  wasEnabled(): Promise<boolean>;
  setEnabled(value: boolean): Promise<void>;
}

export class NotificationRegistrationController {
  state: NotificationRegistrationState = "checking";
  private active = true;

  constructor(
    private readonly projectId: string | null,
    private readonly platform: "ios" | "android",
    private readonly native: NotificationNativeAdapter,
    private readonly subscriptions: DeviceSubscriptionClient,
    private readonly installationId: () => Promise<string>,
    private readonly preference: RegistrationPreference,
    private readonly online: () => boolean,
    private readonly signedIn: () => boolean,
    private readonly onState: (
      state: NotificationRegistrationState,
    ) => void = () => {},
    private readonly binding: () => NotificationBinding = () => ({
      subject: "test-subject",
      sessionId: "test-session",
      workspaceId: "test-workspace",
    }),
    private readonly onRegistered: (
      binding: NotificationBinding,
    ) => void = () => {},
  ) {}

  async inspectAndRepair(): Promise<void> {
    if (!this.projectId) return this.setState("unconfigured");
    if (!this.signedIn() || !this.online()) return this.setState("offline");
    if (!(await this.pushAvailable())) return;
    const permission = await this.native.permission();
    const wasEnabled = await this.preference.wasEnabled();
    if (permission === "undetermined") return this.setState("disabled");
    if (permission === "denied") {
      return this.setState(wasEnabled ? "revoked" : "denied");
    }
    if (!wasEnabled) return this.setState("disabled");
    await this.registerCurrent(true);
  }

  async enableFromUserGesture(): Promise<void> {
    if (!this.projectId) return this.setState("unconfigured");
    this.setState("enabling");
    try {
      if (!this.signedIn() || !this.online()) return this.setState("offline");
      if (!(await this.pushAvailable())) return;
      await this.native.ensureAndroidChannel();
      const permission = await this.native.requestPermission();
      if (permission !== "granted") {
        return this.setState(permission === "denied" ? "denied" : "disabled");
      }
      await this.preference.setEnabled(true);
      await this.registerCurrent(true);
    } catch {
      this.setState("error");
    }
  }

  async repairAfterTokenRotation(): Promise<void> {
    if (!this.signedIn() || !this.online()) return;
    try {
      if ((await this.native.permission()) !== "granted") return;
      await this.registerCurrent();
    } catch {
      // Rotation repair is deliberately best-effort; reconnect also repairs.
    }
  }

  async unregisterCurrent(accessToken?: string): Promise<void> {
    // Remote unregister must succeed before the local preference flips: doing it
    // the other way round leaves the server-side device row bound to this
    // subject forever on a failure, while the local "disabled" state makes
    // inspectAndRepair refuse to ever re-register it (w4/029.md #15).
    const deviceId = await this.installationId();
    await this.subscriptions.unregister(deviceId, accessToken);
    await this.preference.setEnabled(false).catch(() => undefined);
    this.setState("disabled");
  }

  dispose(): void {
    this.active = false;
  }

  private async registerCurrent(serverAvailable = false): Promise<void> {
    if (!this.projectId) return;
    if (!serverAvailable && !(await this.pushAvailable())) return;
    await this.native.ensureAndroidChannel();
    const token = await this.native.expoToken(this.projectId);
    const deviceId = await this.installationId();
    const binding = this.binding();
    await this.subscriptions.register({
      deviceId,
      sessionId: binding.sessionId,
      provider: "expo",
      platform: this.platform,
      token,
    });
    if (this.active) this.onRegistered(binding);
    await this.preference.setEnabled(true);
    this.setState("enabled");
  }

  private async pushAvailable(): Promise<boolean> {
    const availability = await this.subscriptions.list();
    if (availability.available) return true;
    this.setState("unavailable");
    return false;
  }

  private setState(state: NotificationRegistrationState): void {
    this.state = state;
    if (this.active) this.onState(state);
  }
}

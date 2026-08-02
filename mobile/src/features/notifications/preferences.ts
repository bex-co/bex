import type { KeyValueStorage } from "./inbox-store";
import type { RegistrationPreference } from "./registration-controller";

const key = "bex.notifications.enabled.v1";

export class NotificationRegistrationPreference implements RegistrationPreference {
  constructor(private readonly storage: KeyValueStorage) {}

  async wasEnabled(): Promise<boolean> {
    return (await this.storage.getItem(key).catch(() => null)) === "1";
  }

  async setEnabled(value: boolean): Promise<void> {
    if (value) await this.storage.setItem(key, "1");
    else await this.storage.removeItem(key);
  }
}

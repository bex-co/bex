export interface SecretStorage {
  getItemAsync(key: string): Promise<string | null>;
  setItemAsync(key: string, value: string): Promise<void>;
}

const installationKey = "bex.notification.installation.v1";
const validInstallationId = /^[0-9a-f-]{36}$/i;

export class NotificationInstallationStore {
  constructor(
    private readonly storage: SecretStorage,
    private readonly generateId: () => string,
  ) {}

  async getOrCreate(): Promise<string> {
    const existing = await this.storage.getItemAsync(installationKey);
    if (existing && validInstallationId.test(existing)) return existing;
    const created = this.generateId();
    if (!validInstallationId.test(created)) {
      throw new Error("invalid notification installation id");
    }
    await this.storage.setItemAsync(installationKey, created);
    return created;
  }
}

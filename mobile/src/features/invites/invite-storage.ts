import type { SecureStoreOptions } from "expo-secure-store";
import { parseStoredInvite, type StoredInvite } from "./invite-token";

const STORAGE_KEY = "bex.mobile.invite.v1";
export const INVITE_KEYCHAIN_SERVICE = "co.bex.mobile.invite";

export interface InviteSecretStorage {
  isAvailableAsync(): Promise<boolean>;
  getItemAsync(
    key: string,
    options?: SecureStoreOptions,
  ): Promise<string | null>;
  setItemAsync(
    key: string,
    value: string,
    options?: SecureStoreOptions,
  ): Promise<void>;
  deleteItemAsync(key: string, options?: SecureStoreOptions): Promise<void>;
}

export interface InviteStore {
  load(): Promise<StoredInvite | null>;
  save(invite: StoredInvite): Promise<void>;
  clear(): Promise<void>;
}

export class InviteStorageError extends Error {
  constructor() {
    super("invite secure storage unavailable");
    this.name = "InviteStorageError";
  }
}

export class SecureInviteStore implements InviteStore {
  constructor(
    private readonly storage: InviteSecretStorage,
    private readonly options: SecureStoreOptions,
  ) {}

  async load(): Promise<StoredInvite | null> {
    try {
      await this.requireAvailable();
      const encoded = await this.storage.getItemAsync(
        STORAGE_KEY,
        this.options,
      );
      if (!encoded) return null;
      const invite = parseStoredInvite(JSON.parse(encoded));
      if (!invite) await this.clear();
      return invite;
    } catch (error) {
      if (error instanceof InviteStorageError) throw error;
      await this.clear().catch(() => undefined);
      throw new InviteStorageError();
    }
  }

  async save(invite: StoredInvite): Promise<void> {
    try {
      await this.requireAvailable();
      await this.storage.setItemAsync(
        STORAGE_KEY,
        JSON.stringify(invite),
        this.options,
      );
    } catch {
      await this.clear().catch(() => undefined);
      throw new InviteStorageError();
    }
  }

  async clear(): Promise<void> {
    await this.storage.deleteItemAsync(STORAGE_KEY, this.options);
  }

  private async requireAvailable(): Promise<void> {
    if (!(await this.storage.isAvailableAsync())) {
      throw new InviteStorageError();
    }
  }
}

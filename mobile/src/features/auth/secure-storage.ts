import * as SecureStore from "expo-secure-store";
import { AuthFailure } from "./errors";
import { mobileConfig } from "./config";
import { parseStoredSession } from "./session-validation";
import type { AuthStorage, StoredSession } from "./types";

const STORAGE_KEY = "bex.mobile.oauth.v1";
const OPTIONS: SecureStore.SecureStoreOptions = {
  keychainAccessible: SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY,
  keychainService: "co.bex.mobile.oauth",
};

export class SecureAuthStorage implements AuthStorage {
  async load(): Promise<StoredSession | null> {
    try {
      if (!(await SecureStore.isAvailableAsync())) {
        throw new AuthFailure("storage");
      }
      const encoded = await SecureStore.getItemAsync(STORAGE_KEY, OPTIONS);
      if (!encoded) return null;
      const session = parseStoredSession(JSON.parse(encoded), {
        issuer: mobileConfig.oauthIssuer,
        clientId: mobileConfig.oauthClientId,
      });
      if (!session) await this.clear();
      return session;
    } catch (error) {
      if (error instanceof AuthFailure) throw error;
      await this.clear().catch(() => undefined);
      throw new AuthFailure("storage");
    }
  }

  async save(session: StoredSession): Promise<void> {
    try {
      if (!(await SecureStore.isAvailableAsync())) {
        throw new AuthFailure("storage");
      }
      await SecureStore.setItemAsync(
        STORAGE_KEY,
        JSON.stringify(session),
        OPTIONS,
      );
    } catch {
      await this.clear().catch(() => undefined);
      throw new AuthFailure("storage");
    }
  }

  async clear(): Promise<void> {
    await SecureStore.deleteItemAsync(STORAGE_KEY, OPTIONS);
  }
}

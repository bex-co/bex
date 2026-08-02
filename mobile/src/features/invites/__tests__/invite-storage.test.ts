import fs from "fs";
import path from "path";
import {
  INVITE_KEYCHAIN_SERVICE,
  SecureInviteStore,
  type InviteSecretStorage,
} from "../invite-storage";

const token = "0123456789abcdef0123456789abcdef";
const options = {
  keychainAccessible: 123,
  keychainService: INVITE_KEYCHAIN_SERVICE,
};

class SecretStorage implements InviteSecretStorage {
  value: string | null = null;
  available = true;
  setError: Error | null = null;
  calls: { method: string; key: string; options: unknown }[] = [];
  async isAvailableAsync() {
    return this.available;
  }
  async getItemAsync(key: string, options?: unknown) {
    this.calls.push({ method: "get", key, options });
    return this.value;
  }
  async setItemAsync(key: string, value: string, options?: unknown) {
    this.calls.push({ method: "set", key, options });
    if (this.setError) throw this.setError;
    this.value = value;
  }
  async deleteItemAsync(key: string, options?: unknown) {
    this.calls.push({ method: "delete", key, options });
    this.value = null;
  }
}

describe("invite device-only storage", () => {
  it("configures Expo for unlocked, non-migrating storage", () => {
    const source = fs.readFileSync(
      path.resolve(
        process.cwd(),
        "src/features/invites/expo-invite-storage.ts",
      ),
      "utf8",
    );
    expect(source.includes("SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY")).toBe(
      true,
    );
    expect(source.includes("INVITE_KEYCHAIN_SERVICE")).toBe(true);
  });

  it("uses one distinct keychain service and device-only accessibility", async () => {
    const secret = new SecretStorage();
    const store = new SecureInviteStore(secret, options);
    await store.save({ version: 1, token, subject: null });
    expect(JSON.parse(secret.value ?? "null")).toEqual({
      version: 1,
      token,
      subject: null,
    });
    expect(await store.load()).toEqual({ version: 1, token, subject: null });
    await store.clear();
    expect(secret.calls.map((call) => call.method)).toEqual([
      "set",
      "get",
      "delete",
    ]);
    expect(secret.calls.every((call) => call.options === options)).toBe(true);
    expect(INVITE_KEYCHAIN_SERVICE).toBe("co.bex.mobile.invite");
    expect(INVITE_KEYCHAIN_SERVICE).not.toBe("co.bex.mobile.oauth");
  });

  it("clears malformed persisted data instead of exposing it", async () => {
    const secret = new SecretStorage();
    secret.value = JSON.stringify({ version: 1, token: "bad", subject: null });
    const store = new SecureInviteStore(secret, options);
    expect(await store.load()).toBe(null);
    expect(secret.value === null).toBe(true);
  });

  it("fails closed when secure storage is unavailable", async () => {
    const secret = new SecretStorage();
    secret.available = false;
    const store = new SecureInviteStore(secret, options);
    let failed = false;
    try {
      await store.save({ version: 1, token, subject: null });
    } catch {
      failed = true;
    }
    expect(failed).toBe(true);
    expect(secret.value === null).toBe(true);
  });

  it("clears an older bearer when replacement storage fails", async () => {
    const secret = new SecretStorage();
    secret.value = JSON.stringify({ version: 1, token, subject: "identity-a" });
    secret.setError = new Error("write denied");
    const store = new SecureInviteStore(secret, options);
    let failed = false;
    try {
      await store.save({
        version: 1,
        token: "abcdef0123456789abcdef0123456789",
        subject: "identity-b",
      });
    } catch {
      failed = true;
    }
    expect(failed).toBe(true);
    expect(secret.value === null).toBe(true);
    expect(secret.calls.map((call) => call.method)).toEqual(["set", "delete"]);
  });
});

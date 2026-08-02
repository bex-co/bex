import * as SecureStore from "expo-secure-store";
import { INVITE_KEYCHAIN_SERVICE, SecureInviteStore } from "./invite-storage";

export function createExpoInviteStore(): SecureInviteStore {
  return new SecureInviteStore(SecureStore, {
    keychainAccessible: SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY,
    keychainService: INVITE_KEYCHAIN_SERVICE,
  });
}

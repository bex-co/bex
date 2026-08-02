import type { PermissionState } from "./registration-controller";

type PermissionSnapshot = {
  status: string;
  ios?: { status: number };
};

// UNAuthorizationStatus values exposed by expo-notifications. Keep this helper
// native-module-free so permission semantics stay unit-testable.
const iosNotDetermined = 0;
const iosDenied = 1;
const iosAuthorized = new Set([2, 3, 4]); // authorized, provisional, ephemeral

export function notificationPermissionState(
  snapshot: PermissionSnapshot,
  platform: string,
): PermissionState {
  if (platform === "ios" && snapshot.ios) {
    if (iosAuthorized.has(snapshot.ios.status)) return "granted";
    if (snapshot.ios.status === iosDenied) return "denied";
    if (snapshot.ios.status === iosNotDetermined) return "undetermined";
    return "undetermined";
  }
  return snapshot.status === "granted"
    ? "granted"
    : snapshot.status === "denied"
      ? "denied"
      : "undetermined";
}

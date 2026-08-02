import { Platform } from "react-native";
import * as Notifications from "expo-notifications";
import type {
  NotificationNativeAdapter,
  PermissionState,
} from "./registration-controller";
import { notificationPermissionState } from "./permission-state";

export const notificationChannelId = "bex-alerts";

export class ExpoNotificationAdapter implements NotificationNativeAdapter {
  async permission(): Promise<PermissionState> {
    return notificationPermissionState(
      await Notifications.getPermissionsAsync(),
      Platform.OS,
    );
  }

  async requestPermission(): Promise<PermissionState> {
    return notificationPermissionState(
      await Notifications.requestPermissionsAsync({
        ios: { allowAlert: true, allowBadge: true, allowSound: true },
      }),
      Platform.OS,
    );
  }

  async ensureAndroidChannel(): Promise<void> {
    if (Platform.OS !== "android") return;
    await Notifications.setNotificationChannelAsync(notificationChannelId, {
      name: "bex alerts",
      importance: Notifications.AndroidImportance.HIGH,
      showBadge: true,
    });
  }

  async expoToken(projectId: string): Promise<string> {
    return (await Notifications.getExpoPushTokenAsync({ projectId })).data;
  }
}

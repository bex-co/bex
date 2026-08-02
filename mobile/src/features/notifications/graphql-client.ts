import type { ApolloClient } from "@apollo/client";
import {
  MobileNotificationDeviceSubscriptionsDocument,
  MobileNotificationInboxDocument,
  MobileMarkPushNotificationReadDocument,
  MobileRegisterNotificationDeviceSubscriptionDocument,
  MobileUnregisterNotificationDeviceSubscriptionDocument,
} from "@/generated-graphql";
import type { DeviceSubscriptionClient } from "./registration-controller";
import type { RemoteNotificationInboxItem } from "./inbox-store";

export class ApolloNotificationSubscriptionClient implements DeviceSubscriptionClient {
  constructor(private readonly client: ApolloClient) {}

  async list(): Promise<{ available: boolean }> {
    const result = await this.client.query({
      query: MobileNotificationDeviceSubscriptionsDocument,
      fetchPolicy: "network-only",
    });
    return { available: result.data?.pushNotificationsAvailable === true };
  }

  async register(input: {
    deviceId: string;
    provider: "expo";
    platform: "ios" | "android";
    token: string;
  }): Promise<void> {
    await this.client.mutate({
      mutation: MobileRegisterNotificationDeviceSubscriptionDocument,
      variables: input,
    });
  }

  async unregister(deviceId: string, accessToken?: string): Promise<void> {
    await this.client.mutate({
      mutation: MobileUnregisterNotificationDeviceSubscriptionDocument,
      variables: { deviceId },
      context: accessToken
        ? {
            headers: { authorization: `Bearer ${accessToken}` },
            skipAuthRefresh: true,
          }
        : undefined,
    });
  }

  async inbox(limit = 100): Promise<RemoteNotificationInboxItem[]> {
    const result = await this.client.query({
      query: MobileNotificationInboxDocument,
      variables: { limit },
      fetchPolicy: "network-only",
    });
    return (result.data?.notificationInbox ?? []).map((item) => ({
      id: item.id,
      event: String(item.event).toLowerCase(),
      route: item.deepLink,
      title: item.title,
      body: item.body,
      receivedAt: Date.parse(item.occurredAt),
      read: item.readAt != null,
    }));
  }

  async markNotificationRead(id: string): Promise<void> {
    await this.client.mutate({
      mutation: MobileMarkPushNotificationReadDocument,
      variables: { id },
    });
  }
}

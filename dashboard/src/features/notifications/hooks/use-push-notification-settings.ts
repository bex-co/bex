import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  PushNotificationEvent,
  PushNotificationSettingsDocument,
  PushNotificationUrgency,
  type PushNotificationSettingsInput,
} from "@/graphql/definitions";

export const defaultPushNotificationSettings: PushNotificationSettingsInput = {
  enabled: true,
  events: [
    PushNotificationEvent.DeployFailed,
    PushNotificationEvent.ServerFailed,
    PushNotificationEvent.CronFailed,
  ],
  minimumUrgency: PushNotificationUrgency.Important,
  timeZone: "UTC",
  workingHours: [],
  quietHours: [],
  maxDeferralSeconds: 8 * 60 * 60,
  serviceOverrides: [],
};

export function clonePushSettings(
  settings: PushNotificationSettingsInput,
): PushNotificationSettingsInput {
  return {
    ...settings,
    events: [...settings.events],
    workingHours: settings.workingHours.map((range) => ({
      ...range,
      weekdays: [...range.weekdays],
    })),
    quietHours: settings.quietHours.map((range) => ({
      ...range,
      weekdays: [...range.weekdays],
    })),
    serviceOverrides: settings.serviceOverrides.map((override) => ({
      ...override,
      events: override.events ? [...override.events] : undefined,
    })),
  };
}

export function usePushNotificationSettings() {
  const { data, loading, error, refetch } = useQuery(
    PushNotificationSettingsDocument,
    { fetchPolicy: "cache-and-network", errorPolicy: "all" },
  );

  const settings = useMemo<PushNotificationSettingsInput>(() => {
    const raw = data?.pushNotificationSettings;
    if (!raw) return clonePushSettings(defaultPushNotificationSettings);
    return {
      enabled: raw.enabled,
      events: [...raw.events],
      minimumUrgency: raw.minimumUrgency,
      timeZone: raw.timeZone,
      workingHours: raw.workingHours.map((range) => ({
        weekdays: [...range.weekdays],
        start: range.start,
        end: range.end,
      })),
      quietHours: raw.quietHours.map((range) => ({
        weekdays: [...range.weekdays],
        start: range.start,
        end: range.end,
      })),
      maxDeferralSeconds: raw.maxDeferralSeconds,
      serviceOverrides: raw.serviceOverrides.map((override) => ({
        serviceId: override.serviceId,
        enabled: override.enabled,
        events: override.events ? [...override.events] : undefined,
        minimumUrgency: override.minimumUrgency,
      })),
    };
  }, [data?.pushNotificationSettings]);

  return {
    settings,
    available: data?.pushNotificationsAvailable ?? false,
    loading: loading && !data?.pushNotificationSettings,
    error,
    refetch,
  };
}

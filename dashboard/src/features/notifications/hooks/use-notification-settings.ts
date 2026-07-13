import { useQuery } from "@apollo/client/react";
import { NotificationSettingsDocument } from "@/graphql/definitions";

export interface NotificationSettingsView {
  deploySucceeded: boolean;
  deployFailed: boolean;
}

const defaults: NotificationSettingsView = {
  deploySucceeded: true,
  deployFailed: true,
};

export interface UseNotificationSettingsResult {
  settings: NotificationSettingsView;
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
}

/**
 * Reads the CALLER's own deploy-notification preferences
 * (backend/internal/notifications, w3/m9) — self-service, no workspaceId arg.
 * A caller who never customized them gets the server's default (both true).
 */
export function useNotificationSettings(): UseNotificationSettingsResult {
  const { data, loading, error, refetch } = useQuery(
    NotificationSettingsDocument,
    { fetchPolicy: "cache-and-network", errorPolicy: "all" },
  );

  const raw = data?.notificationSettings;
  const settings: NotificationSettingsView = raw
    ? {
        deploySucceeded: raw.deploySucceeded ?? defaults.deploySucceeded,
        deployFailed: raw.deployFailed ?? defaults.deployFailed,
      }
    : defaults;

  return { settings, loading, error, refetch };
}

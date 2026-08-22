import { useQuery } from "@apollo/client/react";
import { NotificationSettingsDocument } from "@/graphql/definitions";
import { PRIMED_FETCH_POLICY } from "@/common/lib/fetch-policy";

export interface NotificationSettingsView {
  deployStarted: boolean;
  deploySucceeded: boolean;
  deployFailed: boolean;
}

const defaults: NotificationSettingsView = {
  deployStarted: true,
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
 * A caller who never customized them gets the server's default (all three true).
 */
export function useNotificationSettings(): UseNotificationSettingsResult {
  const { data, loading, error, refetch } = useQuery(
    NotificationSettingsDocument,
    { fetchPolicy: PRIMED_FETCH_POLICY, errorPolicy: "all" },
  );

  const raw = data?.notificationSettings;
  const settings: NotificationSettingsView = raw
    ? {
        deployStarted: raw.deployStarted ?? defaults.deployStarted,
        deploySucceeded: raw.deploySucceeded ?? defaults.deploySucceeded,
        deployFailed: raw.deployFailed ?? defaults.deployFailed,
      }
    : defaults;

  return { settings, loading, error, refetch };
}

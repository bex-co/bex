import { useQuery } from "@apollo/client/react";
import { NotificationSettingsDocument } from "@/graphql/definitions";
import { PRIMED_FETCH_POLICY } from "@/common/lib/fetch-policy";
import { useWorkspace } from "@/features/workspaces/context/hooks";

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
 * Reads the CALLER's own deploy-notification preferences for the CURRENT
 * workspace (backend/internal/notifications, w3/m9 + w4/m128). Preferences are
 * stored and applied per workspace, so the panel asks about the workspace it is
 * showing rather than whichever one the server would pick as the default — the
 * gap that let a member turn deploy email off and keep receiving it from their
 * other workspaces. A caller who never customized them gets the server default.
 */
export function useNotificationSettings(): UseNotificationSettingsResult {
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading, error, refetch } = useQuery(
    NotificationSettingsDocument,
    {
      variables: { ownerId: currentWorkspaceId },
      fetchPolicy: PRIMED_FETCH_POLICY,
      errorPolicy: "all",
    },
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

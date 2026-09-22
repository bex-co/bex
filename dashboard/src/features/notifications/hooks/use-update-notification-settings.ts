import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  NotificationSettingsDocument,
  UpdateNotificationSettingsDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import type { NotificationSettingsView } from "@/features/notifications/hooks/use-notification-settings";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface UseUpdateNotificationSettingsResult {
  update: (settings: NotificationSettingsView) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires a preference toggle to bex-api's `updateNotificationSettings` for the
 * CURRENT workspace (w4/m128 — preferences are per workspace). The mutation
 * writes its response straight into `NotificationSettingsDocument`'s cache
 * entry for that same workspace, so `useNotificationSettings` picks it up as a
 * normal cache update and callers need no follow-up `refetch()`. The cache
 * write carries the variables deliberately: the query is now keyed by
 * `ownerId`, and writing it unkeyed would put one workspace's answer where
 * another workspace's belongs.
 */
export function useUpdateNotificationSettings(): UseUpdateNotificationSettingsResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(UpdateNotificationSettingsDocument, {
    update: (cache, { data }, { variables }) => {
      if (!data?.updateNotificationSettings) return;
      cache.writeQuery({
        query: NotificationSettingsDocument,
        variables: { ownerId: variables?.ownerId ?? null },
        data: { notificationSettings: data.updateNotificationSettings },
      });
    },
  });
  const [busy, setBusy] = useState(false);

  const update = useCallback(
    async (settings: NotificationSettingsView) => {
      setBusy(true);
      try {
        await mutate({
          variables: { ...settings, ownerId: currentWorkspaceId },
        });
        return true;
      } catch (err) {
        toast.error(mutationErrorMessage(err, t("notifications.updateError")));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { update, busy };
}

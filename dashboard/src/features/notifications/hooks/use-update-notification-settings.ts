import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  NotificationSettingsDocument,
  UpdateNotificationSettingsDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import type { NotificationSettingsView } from "@/features/notifications/hooks/use-notification-settings";

export interface UseUpdateNotificationSettingsResult {
  update: (settings: NotificationSettingsView) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires a preference toggle to bex-api's `updateNotificationSettings`. The
 * mutation writes its response straight into `NotificationSettingsDocument`'s
 * cache entry (the query has no variables, so there's exactly one to target)
 * — `useNotificationSettings`'s `useQuery` picks it up as a normal cache
 * update, so callers don't need a follow-up `refetch()` for data the mutation
 * response already contained.
 */
export function useUpdateNotificationSettings(): UseUpdateNotificationSettingsResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(UpdateNotificationSettingsDocument, {
    update: (cache, { data }) => {
      if (!data?.updateNotificationSettings) return;
      cache.writeQuery({
        query: NotificationSettingsDocument,
        data: { notificationSettings: data.updateNotificationSettings },
      });
    },
  });
  const [busy, setBusy] = useState(false);

  const update = useCallback(
    async (settings: NotificationSettingsView) => {
      setBusy(true);
      try {
        await mutate({ variables: settings });
        return true;
      } catch {
        toast.error(t("notifications.updateError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { update, busy };
}

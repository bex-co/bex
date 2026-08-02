import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  PushNotificationSettingsDocument,
  UpdatePushNotificationSettingsDocument,
  type PushNotificationSettingsInput,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export function useUpdatePushNotificationSettings() {
  const { t } = useTranslations();
  const [busy, setBusy] = useState(false);
  const [mutate] = useMutation(UpdatePushNotificationSettingsDocument, {
    update: (cache, { data }) => {
      if (!data?.updatePushNotificationSettings) return;
      const current = cache.readQuery({
        query: PushNotificationSettingsDocument,
      });
      cache.writeQuery({
        query: PushNotificationSettingsDocument,
        data: {
          pushNotificationsAvailable:
            current?.pushNotificationsAvailable ?? false,
          pushNotificationSettings: data.updatePushNotificationSettings,
        },
      });
    },
  });

  const update = useCallback(
    async (settings: PushNotificationSettingsInput) => {
      setBusy(true);
      try {
        await mutate({ variables: { settings } });
        toast.success(t("notifications.pushSaved"));
        return true;
      } catch {
        toast.error(t("notifications.pushUpdateError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { update, busy };
}

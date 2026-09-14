import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  PushNotificationSettingsDocument,
  UpdatePushNotificationSettingsDocument,
  type PushNotificationSettingsInput,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

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
          webPushAvailable: current?.webPushAvailable ?? false,
          webPushVapidPublicKey: current?.webPushVapidPublicKey ?? null,
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
      } catch (err) {
        toast.error(
          mutationErrorMessage(err, t("notifications.pushUpdateError")),
        );
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { update, busy };
}

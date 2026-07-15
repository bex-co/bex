import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetNotificationsToSendDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export function useServiceNotifications() {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetNotificationsToSendDocument);
  const [busy, setBusy] = useState(false);
  const setNotificationsToSend = useCallback(
    async (id: string, value: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, value } });
        toast.success(t("services.notificationsSuccess"));
        return true;
      } catch {
        toast.error(t("services.notificationsError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );
  return { setNotificationsToSend, busy };
}

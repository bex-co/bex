import { SetNotificationsToSendDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export function useServiceNotifications() {
  const { run, busy } = useFieldMutation(
    SetNotificationsToSendDocument,
    (id: string, value: string) => ({ id, value }),
    {
      success: "services.notificationsSuccess",
      error: "services.notificationsError",
    },
  );
  return { setNotificationsToSend: run, busy };
}

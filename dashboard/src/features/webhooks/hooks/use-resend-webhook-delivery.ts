import { useCallback, useRef, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { useTranslations } from "@/common/hooks/use-translations";
import { ResendWebhookDeliveryDocument } from "@/features/webhooks/api/operations";
import { useWorkspace } from "@/features/workspaces/context/hooks";

function newIdempotencyKey(attemptId: string): string {
  const random = globalThis.crypto?.randomUUID?.();
  return random
    ? `dashboard-${random}`
    : `dashboard-${attemptId}-${Date.now().toString(36)}`;
}

export interface UseResendWebhookDeliveryResult {
  resend: (endpointId: string, attemptId: string) => Promise<boolean>;
  resendingAttemptId: string | null;
}

/**
 * Queues one authorized manual attempt. A failed/ambiguous request retains its
 * idempotency key so confirming that same action again cannot reserve two
 * sends; a successful action clears it so a later deliberate resend is new.
 */
export function useResendWebhookDelivery(): UseResendWebhookDeliveryResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(ResendWebhookDeliveryDocument);
  const [resendingAttemptId, setResendingAttemptId] = useState<string | null>(
    null,
  );
  const inFlight = useRef(new Set<string>());
  const idempotencyKeys = useRef(new Map<string, string>());

  const resend = useCallback(
    async (endpointId: string, attemptId: string) => {
      if (!currentWorkspaceId || inFlight.current.has(attemptId)) return false;

      const idempotencyKey =
        idempotencyKeys.current.get(attemptId) ?? newIdempotencyKey(attemptId);
      idempotencyKeys.current.set(attemptId, idempotencyKey);
      inFlight.current.add(attemptId);
      setResendingAttemptId(attemptId);
      try {
        const result = await mutate({
          variables: {
            endpointId,
            attemptId,
            ownerId: currentWorkspaceId,
            idempotencyKey,
          },
        });
        if (!result.data?.resendWebhookDelivery?.id) {
          throw new Error("resend did not return an attempt");
        }
        idempotencyKeys.current.delete(attemptId);
        toast.success(t("webhooks.resendSuccess"));
        return true;
      } catch {
        toast.error(t("webhooks.resendError"));
        return false;
      } finally {
        inFlight.current.delete(attemptId);
        setResendingAttemptId((current) =>
          current === attemptId ? null : current,
        );
      }
    },
    [currentWorkspaceId, mutate, t],
  );

  return { resend, resendingAttemptId };
}

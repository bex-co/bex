import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetIdleTimeoutDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseIdleTimeoutResult {
  /** Fires setIdleTimeout; resolves true on success (toasted either way). */
  setIdleTimeout: (id: string, idleTTLSeconds: number) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings idle-timeout control to bex-api's `setIdleTimeout` (w1/m4.5)
 * — a bex extension that persists `spec.idleTTLSeconds`. Like the plan/scale
 * verbs the mutation is synchronous (it patches the spec and returns); the
 * operator honors the new window on the next idle check, so the toast confirms
 * the setting rather than implying an instant sleep.
 */
export function useIdleTimeout(): UseIdleTimeoutResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetIdleTimeoutDocument);
  const [busy, setBusy] = useState(false);

  const setIdleTimeout = useCallback(
    async (id: string, idleTTLSeconds: number) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, idleTTLSeconds } });
        toast.success(t("services.idleTimeoutSuccess"));
        return true;
      } catch {
        toast.error(t("services.idleTimeoutError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setIdleTimeout, busy };
}

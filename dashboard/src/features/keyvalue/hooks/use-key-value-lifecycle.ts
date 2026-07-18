import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  SuspendKeyValueDocument,
  ResumeKeyValueDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  protectedConfirmationFromError,
  type ProtectedActionResult,
} from "@/features/services/lib/protected-confirmation";

export type KeyValueLifecycleAction = "suspend" | "resume";

export interface UseKeyValueLifecycleResult {
  pending: KeyValueLifecycleAction | null;
  /** Fires suspendKeyValue/resumeKeyValue; resolves true on success (toasted either way). */
  run: (
    action: KeyValueLifecycleAction,
    id: string,
    name: string,
    confirmation?: string,
  ) => Promise<ProtectedActionResult>;
}

const SUCCESS_KEY: Record<KeyValueLifecycleAction, string> = {
  suspend: "keyvalue.toastSuspendSuccess",
  resume: "keyvalue.toastResumeSuccess",
};

const ERROR_KEY: Record<KeyValueLifecycleAction, string> = {
  suspend: "keyvalue.toastSuspendError",
  resume: "keyvalue.toastResumeError",
};

/**
 * Wires the detail page's suspend/resume actions to bex-api's
 * `suspendKeyValue`/`resumeKeyValue` (w2/m7 — scale-to-zero via
 * `spec.suspended`, captured in Render's own KV detail page as "Suspend Key
 * Value Instance"). The mutation returns the fresh `{id, suspended}`
 * immediately; the caller refetches the detail query afterward so the header
 * picks up the operator's converged state, mirroring `useKeyValue`'s own poll.
 */
export function useKeyValueLifecycle(): UseKeyValueLifecycleResult {
  const { t } = useTranslations();
  const [suspendMutate] = useMutation(SuspendKeyValueDocument);
  const [resumeMutate] = useMutation(ResumeKeyValueDocument);
  const [pending, setPending] = useState<KeyValueLifecycleAction | null>(null);

  const run = useCallback(
    async (
      action: KeyValueLifecycleAction,
      id: string,
      name: string,
      confirmation?: string,
    ) => {
      setPending(action);
      try {
        if (action === "suspend")
          await suspendMutate({ variables: { id, confirm: confirmation } });
        else await resumeMutate({ variables: { id } });
        toast.success(t(SUCCESS_KEY[action], { name }));
        return { status: "success" } as const;
      } catch (err) {
        const requiredConfirmation = protectedConfirmationFromError(err);
        if (requiredConfirmation) {
          return {
            status: "confirmation_required",
            confirmation: requiredConfirmation,
          } as const;
        }
        toast.error(t(ERROR_KEY[action], { name }));
        return { status: "error" } as const;
      } finally {
        setPending(null);
      }
    },
    [suspendMutate, resumeMutate, t],
  );

  return { pending, run };
}

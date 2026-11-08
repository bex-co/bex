import { useCallback } from "react";
import { toast } from "sonner";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useAutomationMutations } from "@/features/agent-automations/hooks/use-agent-automation";
import {
  outcomeDisplay,
  pauseToggleKind,
} from "@/features/agent-automations/lib/display";

interface Target {
  id: string;
  name: string;
  enabled: boolean;
  pausedReason?: string | null;
}

export interface AutomationOperations {
  /** Pause an active automation, resume a paused one. Resolves true on success. */
  toggle: (a: Target) => Promise<boolean>;
  runNow: (a: Target) => Promise<boolean>;
  takeOver: (a: Target) => Promise<boolean>;
  remove: (a: Target) => Promise<boolean>;
  busy: boolean;
}

/**
 * The list's switch and the detail page's actions, with their toasts. A
 * refusal shows the server's own reason (a resume that fails template
 * validation says why) instead of a generic failure.
 */
export function useAutomationOperations(): AutomationOperations {
  const { t } = useTranslations();
  const m = useAutomationMutations();

  const attempt = useCallback(
    async (work: () => Promise<string | null>): Promise<boolean> => {
      try {
        const success = await work();
        if (success) toast.success(success);
        return true;
      } catch (err) {
        toast.error(
          mutationErrorMessage(err, t("agentAutomations.actionFailed")),
        );
        return false;
      }
    },
    [t],
  );

  const toggle = useCallback(
    (a: Target) =>
      attempt(async () => {
        if (pauseToggleKind(a) === "pause") {
          await m.pause(a.id);
          return t("agentAutomations.pausedToast", { name: a.name });
        }
        await m.resume(a.id);
        return t("agentAutomations.resumedToast", { name: a.name });
      }),
    [attempt, m, t],
  );
  const runNow = useCallback(
    (a: Target) =>
      attempt(async () => {
        const run = await m.runNow(a.id);
        const outcome = outcomeDisplay(run.outcome);
        return run.outcome === "started"
          ? t("agentAutomations.runStartedToast", { name: a.name })
          : t("agentAutomations.runSettledToast", {
              name: a.name,
              outcome: outcome.known ? t(outcome.labelKey) : run.outcome,
            });
      }),
    [attempt, m, t],
  );
  const takeOver = useCallback(
    (a: Target) =>
      attempt(async () => {
        await m.takeOver(a.id);
        return t("agentAutomations.tookOverToast", { name: a.name });
      }),
    [attempt, m, t],
  );
  const remove = useCallback(
    (a: Target) =>
      attempt(async () => {
        await m.remove(a.id);
        return t("agentAutomations.deletedToast", { name: a.name });
      }),
    [attempt, m, t],
  );

  return { toggle, runNow, takeOver, remove, busy: m.busy };
}

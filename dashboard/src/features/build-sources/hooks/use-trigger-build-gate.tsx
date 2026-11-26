import type { ReactNode } from "react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { PipelineMinutesReason } from "@/features/usage/components/pipeline-minutes-reason";
import { usePipelineMinutesExceeded } from "@/features/usage/hooks/use-pipeline-minutes-exceeded";

/**
 * Whether a build source's Trigger Build may be used, and the reason it may
 * not. Render disables a build source's Manual Build while the workspace's
 * pipeline minutes are spent, with that notice ahead of a missing permission
 * (its `pfe`, w3/108).
 */
export function useTriggerBuildGate(): {
  allowed: boolean;
  reason: ReactNode;
} {
  const { t } = useTranslations();
  const capabilities = useCapabilities();
  const minutesSpent = usePipelineMinutesExceeded();
  if (minutesSpent) {
    return { allowed: false, reason: <PipelineMinutesReason /> };
  }
  const reasonKey = capabilities.reasonKey("can_create");
  return {
    allowed: capabilities.canCreate,
    reason: reasonKey ? t(reasonKey) : undefined,
  };
}

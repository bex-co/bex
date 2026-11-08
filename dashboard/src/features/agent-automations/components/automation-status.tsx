import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  automationStatus,
  pausedReasonKey,
} from "@/features/agent-automations/lib/display";

/**
 * The paused badge: "Paused" for a person's pause, "Paused: repeated
 * failures" and the like when the platform paused it (ADR101 D3, D4). Nothing
 * for an active automation.
 */
export function AutomationStatusBadge({
  automation,
}: {
  automation: {
    enabled: boolean;
    pausedReason?: string | null;
    nextFireAt?: string | null;
  };
}) {
  const { t } = useTranslations();
  const status = automationStatus(automation);
  if (status === "active") return null;
  if (status === "ended") {
    return <Badge variant="outline">{t("agentAutomations.ended")}</Badge>;
  }
  const reasonKey = pausedReasonKey(automation.pausedReason);
  return (
    <Badge
      variant={reasonKey ? "destructive" : "secondary"}
      data-paused-reason={automation.pausedReason ?? ""}
      className="max-w-full"
    >
      <span className="truncate">
        {reasonKey ? t(reasonKey) : t("agentAutomations.pausedBadge")}
      </span>
    </Badge>
  );
}

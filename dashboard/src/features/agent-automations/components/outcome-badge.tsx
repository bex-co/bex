import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  outcomeDisplay,
  type RunSessionStatus,
} from "@/features/agent-automations/lib/display";

/**
 * One run's outcome (ADR101 D3/D7). An outcome this build does not know shows
 * the server's raw word under a neutral badge instead of a blank. A started
 * run whose session waits for a sandbox machine says so, and one whose
 * session's sandbox did not start reads "Didn't start", in outline, apart from
 * a failed create's solid "Failed" (w10/m497).
 */
export function OutcomeBadge({
  outcome,
  status = null,
}: {
  outcome: string;
  status?: RunSessionStatus | null;
}) {
  const { t } = useTranslations();
  if (status?.kind === "waiting") {
    return (
      <Badge
        variant="secondary"
        data-outcome={outcome}
        data-run-session={status.kind}
      >
        {t("agentSessions.statusPhrase.waitingForSandbox")}
      </Badge>
    );
  }
  if (status?.kind === "did_not_start") {
    return (
      <Badge
        variant="outline"
        className="border-destructive/50 text-destructive"
        data-outcome={outcome}
        data-run-session={status.kind}
      >
        {t("agentAutomations.outcome.didNotStart")}
      </Badge>
    );
  }
  const display = outcomeDisplay(outcome);
  return (
    <Badge variant={display.variant} data-outcome={outcome}>
      {display.known ? t(display.labelKey) : outcome}
    </Badge>
  );
}

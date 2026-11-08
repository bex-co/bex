import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import { outcomeDisplay } from "@/features/agent-automations/lib/display";

/**
 * One run's outcome (ADR101 D3/D7). An outcome this build does not know shows
 * the server's raw word under a neutral badge instead of a blank.
 */
export function OutcomeBadge({ outcome }: { outcome: string }) {
  const { t } = useTranslations();
  const display = outcomeDisplay(outcome);
  return (
    <Badge variant={display.variant} data-outcome={outcome}>
      {display.known ? t(display.labelKey) : outcome}
    </Badge>
  );
}

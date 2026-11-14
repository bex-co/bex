import { CircleStop } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The end of a turn a person stopped with Stop turn (ADR047 D12): a muted
 * system line in the transcript, in the voice of the other turn annotations and
 * not the failure red, because the session did not fail. The reply above it is
 * what the agent had streamed when it was stopped.
 */
export function TurnInterruptedNote({ turn }: { turn: number }) {
  const { t } = useTranslations();
  return (
    <div
      data-testid="turn-interrupted"
      className="text-muted-foreground flex items-start gap-2 pt-2 text-xs"
    >
      <CircleStop aria-hidden className="mt-0.5 size-3 shrink-0" />
      <p className="min-w-0">{t("agentSessions.turnInterrupted", { turn })}</p>
    </div>
  );
}

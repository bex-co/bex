import { CircleStop } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useLocalDateTime } from "@/common/hooks/use-local-date";
import type {
  TurnRetentionLostReason,
  TurnRetentionView,
} from "@/features/agent-sessions/types";

/**
 * The end of a turn a person stopped with Stop turn (ADR047 D12): a muted
 * system line in the transcript, in the voice of the other turn annotations and
 * not the failure red, because the session did not fail. The reply above it is
 * what the agent had streamed when it was stopped. Beneath it, what became of
 * the unpushed work (w22/m413): kept until a time, being saved, continued by
 * the next message, or lost and why; nothing at all for a turn that kept
 * nothing.
 */
export function TurnInterruptedNote({
  turn,
  retention,
}: {
  turn: number;
  retention?: TurnRetentionView;
}) {
  const { t } = useTranslations();
  return (
    <div
      data-testid="turn-interrupted"
      className="text-muted-foreground flex items-start gap-2 pt-2 text-xs"
    >
      <CircleStop aria-hidden className="mt-0.5 size-3 shrink-0" />
      <div className="min-w-0 space-y-0.5">
        <p>{t("agentSessions.turnInterrupted", { turn })}</p>
        {retention ? <TurnRetentionLine retention={retention} /> : null}
      </div>
    </div>
  );
}

const LOST_REASON_KEYS: Record<TurnRetentionLostReason, string> = {
  expired: "agentSessions.turnRetentionLostExpired",
  storage_full: "agentSessions.turnRetentionLostStorageFull",
  save_failed: "agentSessions.turnRetentionLostSaveFailed",
  editor_open: "agentSessions.turnRetentionLostEditorOpen",
  not_restored: "agentSessions.turnRetentionLostNotRestored",
  // The session ended, which the page already shows: a loss with no cause of
  // its own to name.
  canceled: "agentSessions.turnRetentionLost",
  archived: "agentSessions.turnRetentionLost",
};

/**
 * What became of a stopped turn's unpushed work, in one line. A kept disk's
 * time is the viewer's own clock, so it is not drawn before hydration (the SSR
 * container's would be wrong): the line then says it is kept without the time.
 */
function TurnRetentionLine({ retention }: { retention: TurnRetentionView }) {
  const { t } = useTranslations();
  const until = useLocalDateTime(retention.until);
  let text: string;
  switch (retention.state) {
    case "saving":
      text = t("agentSessions.turnRetentionSaving");
      break;
    case "kept":
      text = until
        ? t("agentSessions.turnRetentionKept", { time: until })
        : t("agentSessions.turnRetentionKeptNoTime");
      break;
    case "continued":
      text = t("agentSessions.turnRetentionContinued");
      break;
    case "lost":
      text = t(
        retention.lostReason
          ? LOST_REASON_KEYS[retention.lostReason]
          : "agentSessions.turnRetentionLost",
      );
      break;
    default:
      return null;
  }
  return (
    <p data-testid="turn-retention" data-retention={retention.state}>
      {text}
    </p>
  );
}

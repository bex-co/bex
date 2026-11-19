import { CircleStop, HardDrive } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useLocalDateTime } from "@/common/hooks/use-local-date";
import type {
  TurnEndView,
  TurnRetentionLostReason,
  TurnRetentionView,
} from "@/features/agent-sessions/types";

/**
 * The end of a turn that has a note: a muted system line in the transcript, in
 * the voice of the other turn annotations and not the failure red. For a turn
 * a person stopped with Stop turn (ADR047 D12) it says so, because the session
 * did not fail; the reply above it is what the agent had streamed when it was
 * stopped. Beneath it, what became of the work the sandbox held undelivered
 * (w22/m413, w10/m491): kept until a time, being saved, continued by the next
 * message, or lost and why, with what was at stake when bex-api counted it. A
 * turn nobody stopped has only that line, and one that kept nothing has none.
 */
export function TurnInterruptedNote({
  turn,
  retention,
  interrupted = true,
}: {
  turn: number;
  retention: TurnRetentionView;
  interrupted?: TurnEndView["interrupted"];
}) {
  const { t } = useTranslations();
  const Icon = interrupted ? CircleStop : HardDrive;
  return (
    <div
      data-testid={interrupted ? "turn-interrupted" : "turn-retained"}
      className="text-muted-foreground flex items-start gap-2 pt-2 text-xs"
    >
      <Icon aria-hidden className="mt-0.5 size-3 shrink-0" />
      <div className="min-w-0 space-y-0.5">
        {interrupted ? (
          <p>{t("agentSessions.turnInterrupted", { turn })}</p>
        ) : null}
        <TurnRetentionLine retention={retention} />
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
  sandbox_lost: "agentSessions.turnRetentionLostSandboxLost",
  // The session ended, which the page already shows: a loss with no cause of
  // its own to name.
  canceled: "agentSessions.turnRetentionLost",
  archived: "agentSessions.turnRetentionLost",
};

/**
 * What became of a turn's undelivered work, in one line, and what was at stake
 * when bex-api counted it. A kept disk's time is the viewer's own clock, so it
 * is not drawn before hydration (the SSR container's would be wrong): the line
 * then says it is kept without the time.
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
  const stake = [
    retention.unpushedCommits
      ? t("agentSessions.turnStakeCommits", {
          count: retention.unpushedCommits,
        })
      : null,
    retention.uncommittedChanges ? t("agentSessions.turnStakeChanges") : null,
  ].filter(Boolean);
  return (
    <>
      <p data-testid="turn-retention" data-retention={retention.state}>
        {text}
      </p>
      {stake.length > 0 ? (
        <p data-testid="turn-retention-stake">{stake.join(" · ")}</p>
      ) : null}
    </>
  );
}

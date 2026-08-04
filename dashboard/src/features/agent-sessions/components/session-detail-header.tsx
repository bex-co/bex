import { useEffect, useState } from "react";
import { GitBranch, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/common/components/ui/tooltip";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import { agentSessionDurationMs } from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import { AgentSessionPhaseChip } from "@/features/agent-sessions/components/session-list";

/** Compact `h/m/s` elapsed label ("1h 4m", "12m 8s", "3s"). */
function formatDurationShort(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const hours = Math.floor(total / 3600);
  const mins = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  if (hours > 0) return `${hours}h ${mins}m`;
  if (mins > 0) return `${mins}m ${secs}s`;
  return `${secs}s`;
}

export interface SessionDetailHeaderProps {
  session: AgentSessionView;
  /** Re-read the session once a cancel converges (the header owns no cache). */
  onCanceled?: () => void;
}

/**
 * The detail-page header (ADR047 D9): phase chip, repo/branch, and the derived
 * meta row (duration, turns, delivery mode) plus cancel-with-confirm. The
 * duration ticks live while the session is non-terminal; once terminal it pins
 * to the session's own end timestamp (via the mapper). Cancel is offered only
 * while the session can still be stopped and is disabled with a reason once
 * canceling/canceled — the confirm copy states pushed work is preserved.
 */
export function SessionDetailHeader({
  session,
  onCanceled,
}: SessionDetailHeaderProps) {
  const { t } = useTranslations();
  const { cancel } = useAgentSessionMutations();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [canceling, setCanceling] = useState(false);

  // Tick a live clock only while the session is still running; terminal
  // sessions resolve to a fixed elapsed from the mapper and never re-render here.
  const [nowMs, setNowMs] = useState(() => Date.now());
  useEffect(() => {
    if (session.isTerminal) return;
    const timer = setInterval(() => setNowMs(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [session.isTerminal]);

  const duration = formatDurationShort(agentSessionDurationMs(session, nowMs));

  // Cancelable = a session still doing work. `canceling` shows the button
  // disabled with a reason; the terminal completed/failed/canceled states hide
  // it (canceled included — there is nothing left to stop).
  const isCanceling = session.phase === "canceling";
  const cancelable = !session.isTerminal && !isCanceling;
  const showCancel = cancelable || isCanceling;

  async function handleConfirm() {
    setCanceling(true);
    try {
      await cancel(session.id);
      toast.success(t("agentSessions.cancelSuccess"));
      onCanceled?.();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setCanceling(false);
      setConfirmOpen(false);
    }
  }

  return (
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div className="min-w-0 space-y-2">
        <div className="flex flex-wrap items-center gap-2">
          <AgentSessionPhaseChip phase={session.phase} />
          <h1 className="truncate text-xl font-semibold">{session.repo}</h1>
        </div>
        <div className="text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
          <span className="inline-flex items-center gap-1.5 font-mono text-xs">
            <GitBranch className="size-3.5" />
            {session.branch}
          </span>
          <span>{t("agentSessions.metaDuration", { duration })}</span>
          <span>{t("agentSessions.metaTurns", { turns: session.turns })}</span>
          {session.deliveryMode ? (
            <span>
              {t("agentSessions.metaDelivery", {
                mode: t(`agentSessions.delivery.${session.deliveryMode}`),
              })}
            </span>
          ) : null}
        </div>
      </div>

      {showCancel ? (
        <CancelButton
          disabled={!cancelable || canceling}
          reason={
            isCanceling ? t("agentSessions.cancelDisabledCanceling") : null
          }
          onClick={() => setConfirmOpen(true)}
        />
      ) : null}

      <AlertDialog
        open={confirmOpen}
        onOpenChange={(open) => !open && !canceling && setConfirmOpen(false)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("agentSessions.cancelConfirmTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("agentSessions.cancelConfirmBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={canceling}>
              {t("agentSessions.cancelConfirmDismiss")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                void handleConfirm();
              }}
              disabled={canceling}
            >
              {canceling ? (
                <>
                  <Loader2 className="animate-spin" />
                  {t("agentSessions.canceling")}
                </>
              ) : (
                t("agentSessions.cancelConfirmProceed")
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

/** The cancel affordance; a disabled button wears its reason in a tooltip. */
function CancelButton({
  disabled,
  reason,
  onClick,
}: {
  disabled: boolean;
  reason: string | null;
  onClick: () => void;
}) {
  const { t } = useTranslations();
  const button = (
    <Button
      size="sm"
      variant="outline"
      disabled={disabled}
      onClick={onClick}
      className="shrink-0"
    >
      {t("agentSessions.cancel")}
    </Button>
  );
  if (!reason) return button;
  return (
    <Tooltip>
      {/* A disabled button swallows pointer events, so wrap it for the tooltip. */}
      <TooltipTrigger asChild>
        <span className="shrink-0" tabIndex={0}>
          {button}
        </span>
      </TooltipTrigger>
      <TooltipContent>{reason}</TooltipContent>
    </Tooltip>
  );
}

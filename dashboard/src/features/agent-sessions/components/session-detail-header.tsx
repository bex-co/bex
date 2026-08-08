import { useEffect, useState } from "react";
import { Link } from "@tanstack/react-router";
import {
  ArrowLeft,
  ExternalLink,
  GitBranch,
  GitPullRequest,
  Loader2,
  MoreHorizontal,
  PanelRightOpen,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
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
  /** Opens the evidence side panel (t006) — the header's panel-toggle target. */
  onOpenEvidence?: () => void;
}

/**
 * The full-page chat header (ADR047 D9, w3/m44): a compact top bar with a
 * back-to-sessions link, the phase chip + repo title, a compact meta row
 * (branch, ticking duration, turns), an inline draft-PR badge `#N`, the evidence
 * panel toggle, a "…" overflow menu (open PR), and cancel-with-confirm. The
 * duration ticks live while the session is non-terminal; once terminal it pins
 * to the session's own end timestamp (via the mapper). Cancel is offered only
 * while the session can still be stopped and is disabled with a reason once
 * canceling/canceled — the confirm copy states pushed work is preserved.
 */
export function SessionDetailHeader({
  session,
  onCanceled,
  onOpenEvidence,
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
    <div className="bg-background/95 supports-backdrop-filter:bg-background/60 flex shrink-0 items-center gap-3 border-b px-4 py-2 backdrop-blur">
      {/* Back to the sessions list (the sidebar handles this on wide screens,
          but the link keeps /agents reachable on mobile where it is hidden). */}
      <Button
        asChild
        size="icon"
        variant="ghost"
        className="shrink-0 lg:hidden"
      >
        <Link to="/agents" aria-label={t("agentSessions.backToList")}>
          <ArrowLeft className="size-4" />
        </Link>
      </Button>

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <AgentSessionPhaseChip phase={session.phase} />
          <h1 className="truncate text-sm font-semibold">{session.repo}</h1>
        </div>
        <div className="text-muted-foreground mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs">
          <span className="inline-flex items-center gap-1 font-mono">
            <GitBranch className="size-3" />
            {session.branch}
          </span>
          <span>{t("agentSessions.metaDuration", { duration })}</span>
          <span>{t("agentSessions.metaTurns", { turns: session.turns })}</span>
          {session.deliveryMode ? (
            <span className="hidden sm:inline">
              {t("agentSessions.metaDelivery", {
                // An unknown/future mode must render as itself, never as a raw
                // i18n key (a stub value once leaked "agentSessions.delivery.…").
                mode: t(`agentSessions.delivery.${session.deliveryMode}`, {
                  defaultValue: session.deliveryMode,
                }),
              })}
            </span>
          ) : null}
        </div>
      </div>

      <HeaderPrBadge session={session} />

      {onOpenEvidence ? (
        <Button
          size="sm"
          variant="outline"
          onClick={onOpenEvidence}
          className="shrink-0"
        >
          <PanelRightOpen className="size-4" />
          <span className="hidden sm:inline">
            {t("agentSessions.evidenceToggle")}
          </span>
        </Button>
      ) : null}

      {showCancel ? (
        <CancelButton
          disabled={!cancelable || canceling}
          reason={
            isCanceling ? t("agentSessions.cancelDisabledCanceling") : null
          }
          onClick={() => setConfirmOpen(true)}
        />
      ) : null}

      {session.prUrl ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              size="icon"
              variant="ghost"
              className="shrink-0"
              aria-label={t("agentSessions.menuMore")}
            >
              <MoreHorizontal className="size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem asChild>
              <a href={session.prUrl} target="_blank" rel="noreferrer">
                <ExternalLink className="size-4" />
                {t("agentSessions.openPr")}
              </a>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
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

/** The inline draft-PR badge `#N` — links `prUrl` in a new tab when present. */
function HeaderPrBadge({ session }: { session: AgentSessionView }) {
  const { t } = useTranslations();
  if (session.prNumber == null) return null;
  const label = t("agentSessions.prBadge", { number: session.prNumber });
  const badge = (
    <Badge variant="secondary" className="hidden gap-1 sm:inline-flex">
      <GitPullRequest className="size-3.5" />
      {label}
    </Badge>
  );
  if (!session.prUrl) return badge;
  return (
    <a
      href={session.prUrl}
      target="_blank"
      rel="noreferrer"
      className="hidden shrink-0 sm:inline-flex"
    >
      <Badge variant="secondary" className="hover:bg-secondary/70 gap-1">
        <GitPullRequest className="size-3.5" />
        {label}
      </Badge>
    </a>
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

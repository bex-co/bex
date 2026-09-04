import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import {
  Archive,
  ArchiveRestore,
  ArrowLeft,
  ChevronDown,
  Code2,
  ExternalLink,
  GitBranch,
  GitPullRequest,
  Loader2,
  MoreHorizontal,
  Pin,
  Terminal,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { CopyButton } from "@/common/components/copy-button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/common/components/ui/tooltip";
import { useTranslations } from "@/common/hooks/use-translations";
import { AddSshKeyCta } from "@/features/ssh-keys/components/add-ssh-key-cta";
import { RequiresSshKey } from "@/features/ssh-keys/components/requires-ssh-key";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import { useArchiveToggle } from "@/features/agent-sessions/hooks/use-archive-toggle";
import { agentSessionErrorMessage } from "@/features/agent-sessions/lib/errors";
import {
  agentSessionDisplayName,
  agentSessionDurationMs,
  formatSnapshotBytes,
} from "@/features/agent-sessions/lib/mapper";
import type {
  AgentSessionListSearch,
  AgentSessionView,
} from "@/features/agent-sessions/types";
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
  /** Re-read the session after a lifecycle action (the header owns no cache). */
  onChanged?: () => void | Promise<unknown>;
  /** List filters to restore when the mobile Back affordance is used. */
  backSearch?: AgentSessionListSearch;
}

/**
 * The full-page chat header (ADR047 D9, w3/m44): a compact top bar with a
 * back-to-sessions link, the phase chip + the session's derived name (w1/m90 —
 * its repo, or its prompt when the session is repo-less), a compact meta row
 * (branch when there is one, ticking duration, turns), an inline draft-PR badge
 * `#N`, the Open in
 * Zed action (w2/m65), a "…" overflow menu (open PR), and cancel-with-confirm. The duration ticks live
 * while the session is non-terminal; once terminal it pins to the session's own
 * end timestamp (via the mapper). Cancel is offered only while the session can
 * still be stopped and is disabled with a reason once canceling/canceled — the
 * confirm copy states pushed work is preserved. The evidence-panel toggle that
 * sat beside the PR badge was removed in w5/m65 along with the panel itself.
 */
export function SessionDetailHeader({
  session,
  onChanged,
  backSearch,
}: SessionDetailHeaderProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { cancel, pin, unpin, deleteSession } = useAgentSessionMutations();
  const handleArchiveChanged = useCallback(async () => {
    if (session.isArchived) {
      await onChanged?.();
      return;
    }
    await navigate({ to: "/agents" });
  }, [navigate, onChanged, session.isArchived]);
  const { toggle: toggleArchive, busyId: archiveBusyId } =
    useArchiveToggle(handleArchiveChanged);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [canceling, setCanceling] = useState(false);
  const [pinning, setPinning] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // Archive is list-state (ADR065 D1) — allowed in any phase; delete is the
  // destructive verb, offered only once no sandbox work is possible
  // (a live session must cancel first).
  const deletable = session.isFinished;

  async function handleDelete() {
    setDeleting(true);
    try {
      await deleteSession(session.id);
      toast.success(t("agentSessions.deleteSuccess"));
      await navigate({ to: "/agents", replace: true });
    } catch (err) {
      toast.error(agentSessionErrorMessage(err, t));
      setDeleting(false);
      setDeleteOpen(false);
    }
  }

  async function handleTogglePin() {
    setPinning(true);
    try {
      if (session.pinned) {
        await unpin(session.id);
        toast.success(t("agentSessions.unpinSuccess"));
      } else {
        await pin(session.id);
        toast.success(t("agentSessions.pinSuccess"));
      }
      await onChanged?.(); // re-read before re-enabling the lifecycle control
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setPinning(false);
    }
  }

  // Tick a live clock only while the session is still running; terminal
  // sessions resolve to a fixed elapsed from the mapper and never re-render here.
  const [nowMs, setNowMs] = useState(() => Date.now());
  useEffect(() => {
    if (session.isTerminal) return;
    const timer = setInterval(() => setNowMs(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [session.isTerminal]);

  const duration = formatDurationShort(agentSessionDurationMs(session, nowMs));

  const displayName = agentSessionDisplayName(
    session,
    t("agentSessions.untitled"),
  );

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
      await onChanged?.();
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
        <Link
          to="/agents"
          search={backSearch}
          aria-label={t("agentSessions.backToList")}
        >
          <ArrowLeft className="size-4" />
        </Link>
      </Button>

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <AgentSessionPhaseChip phase={session.phase} />
          <h1
            className="truncate text-sm font-semibold"
            title={displayName.full}
          >
            {displayName.text}
          </h1>
        </div>
        <div className="text-muted-foreground mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs">
          {/* A repo-less (chat-only) session has no branch — drop the whole
              element, icon included, rather than leaving an orphan GitBranch
              floating next to nothing (w1/m90 t002). */}
          {session.branch ? (
            <span className="inline-flex items-center gap-1 font-mono">
              <GitBranch className="size-3" />
              {session.branch}
            </span>
          ) : null}
          <span>{t("agentSessions.metaDuration", { duration })}</span>
          <span>{t("agentSessions.metaTurns", { count: session.turns })}</span>
          {session.isHibernated ? (
            <span>
              {t("agentSessions.hibernatedStorage", {
                size: formatSnapshotBytes(session.snapshotBytes),
              })}
            </span>
          ) : null}
          {session.pinned ? (
            <Badge variant="outline" className="gap-1">
              <Pin className="size-3" />
              {t("agentSessions.pinned")}
            </Badge>
          ) : null}
          {session.isArchived ? (
            <Badge variant="secondary" className="gap-1">
              <Archive className="size-3" />
              {t("agentSessions.archivedBadge")}
            </Badge>
          ) : null}
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

      <OpenInZedButton session={session} />

      {session.isHibernated ? (
        <Button
          size="sm"
          variant={session.pinned ? "secondary" : "outline"}
          disabled={pinning}
          onClick={handleTogglePin}
        >
          <Pin className="size-4" />
          {session.pinned ? t("agentSessions.unpin") : t("agentSessions.pin")}
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
          {session.prUrl ? (
            <DropdownMenuItem asChild>
              <a href={session.prUrl} target="_blank" rel="noreferrer">
                <ExternalLink className="size-4" />
                {t("agentSessions.openPr")}
              </a>
            </DropdownMenuItem>
          ) : null}
          <DropdownMenuItem
            disabled={archiveBusyId === session.id}
            onClick={() => void toggleArchive(session)}
          >
            {session.isArchived ? (
              <>
                <ArchiveRestore className="size-4" />
                {t("agentSessions.unarchive")}
              </>
            ) : (
              <>
                <Archive className="size-4" />
                {t("agentSessions.archive")}
              </>
            )}
          </DropdownMenuItem>
          {deletable ? (
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setDeleteOpen(true)}
            >
              <Trash2 className="size-4" />
              {t("agentSessions.delete")}
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={(open) => !open && !deleting && setDeleteOpen(false)}
        title={t("agentSessions.deleteConfirmTitle")}
        description={t("agentSessions.deleteConfirmBody")}
        cancelLabel={t("agentSessions.deleteConfirmDismiss")}
        confirmLabel={
          deleting ? (
            <>
              <Loader2 className="animate-spin" />
              {t("agentSessions.deleting")}
            </>
          ) : (
            t("agentSessions.deleteConfirmProceed")
          )
        }
        pending={deleting}
        onConfirm={() => void handleDelete()}
      />

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={(open) => !open && !canceling && setConfirmOpen(false)}
        title={t("agentSessions.cancelConfirmTitle")}
        description={t("agentSessions.cancelConfirmBody")}
        cancelLabel={t("agentSessions.cancelConfirmDismiss")}
        confirmLabel={
          canceling ? (
            <>
              <Loader2 className="animate-spin" />
              {t("agentSessions.canceling")}
            </>
          ) : (
            t("agentSessions.cancelConfirmProceed")
          )
        }
        pending={canceling}
        onConfirm={() => void handleConfirm()}
      />
    </div>
  );
}

/**
 * The "Open in Zed" affordance (ADR054 D5): a Connect-style dropdown offering a
 * `zed://ssh/…/workspace` hotlink that opens the session's sandbox as a Zed
 * remote project, plus a copyable `ssh <address>` command for any other editor
 * or a one-shot shell. Rendered only while the backend surfaces `sshAddress`
 * (BEX_SSH_HOST set AND the sandbox live) — its absence hides the whole control,
 * so a dangling target is never offered. The `zed://` href is an intentional
 * external-protocol link (the first custom scheme in the dashboard); the browser
 * hands it to the OS, which launches Zed.
 */
function OpenInZedButton({ session }: { session: AgentSessionView }) {
  const { t } = useTranslations();
  if (!session.sshAddress) return null;
  const command = `ssh ${session.sshAddress}`;
  const zedUrl = `zed://ssh/${session.sshAddress}/workspace`;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="shrink-0">
          <Code2 className="size-4" />
          <span className="hidden sm:inline">{t("agentSessions.connect")}</span>
          <ChevronDown className="size-3.5" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-80 p-3">
        {/* Gate the doomed payload behind key registration (w2/m66): with no
            key, the zed:// link + ssh command would fail off-surface in Zed's
            own dialog, so swap them for an add-key CTA that returns here. The
            trigger stays visible so the feature is still discoverable. Agent
            sessions have no in-dashboard terminal yet (ADR047 D8 is phase 2), so
            there is no second "browser terminal" door here — only on services. */}
        <RequiresSshKey
          surface="agent-session-zed"
          fallback={
            <AddSshKeyCta
              surface="agent-session-zed"
              returnTo={`/agents/${session.id}`}
            />
          }
        >
          <DropdownMenuItem asChild className="mb-2 cursor-pointer">
            {/* An intentional external-scheme anchor — the OS launches Zed. */}
            <a href={zedUrl}>
              <Code2 className="size-4" />
              {t("agentSessions.openInZed")}
            </a>
          </DropdownMenuItem>
          <DropdownMenuLabel className="flex items-center gap-2 px-0 pt-0">
            <Terminal className="size-4" />
            {t("agentSessions.connectSSH")}
          </DropdownMenuLabel>
          <div className="bg-muted flex min-w-0 items-center gap-1 rounded-md py-1 pr-1 pl-2">
            <code className="min-w-0 flex-1 truncate text-xs" title={command}>
              {command}
            </code>
            <CopyButton
              value={command}
              label={t("agentSessions.sshCopy")}
              successText={t("agentSessions.sshCopied")}
              errorText={t("agentSessions.sshCopyError")}
            />
          </div>
          <p className="text-muted-foreground mt-2 text-xs">
            {t("agentSessions.openInZedHint")}
          </p>
        </RequiresSshKey>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** The inline draft-PR badge `#N` — links `prUrl` in a new tab when present. */
function HeaderPrBadge({ session }: { session: AgentSessionView }) {
  const { t } = useTranslations();
  if (session.prNumber == null || session.prNumber === 0) return null;
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

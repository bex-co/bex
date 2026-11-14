import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import {
  Archive,
  ArchiveRestore,
  ArrowLeft,
  ChevronDown,
  CircleStop,
  Code2,
  ExternalLink,
  GitBranch,
  Loader2,
  MoreHorizontal,
  Tag,
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
import { SessionFilesButton } from "@/features/agent-sessions/components/session-files-sheet";
import { SessionModelCost } from "@/features/agent-sessions/components/session-model-cost";
import { SessionSecrets } from "@/features/agent-sessions/components/session-secrets";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/common/components/ui/tooltip";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspaceSubjectLabel } from "@/common/hooks/use-workspace-subject-label";
import { useAgentAutomation } from "@/features/agent-automations/hooks/use-agent-automation";
import { AddSshKeyCta } from "@/features/ssh-keys/components/add-ssh-key-cta";
import { RequiresSshKey } from "@/features/ssh-keys/components/requires-ssh-key";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import { useArchiveToggle } from "@/features/agent-sessions/hooks/use-archive-toggle";
import { agentSessionErrorMessage } from "@/features/agent-sessions/lib/errors";
import {
  agentSessionDisplayName,
  agentSessionDurationMs,
} from "@/features/agent-sessions/lib/mapper";
import type {
  AgentSessionListSearch,
  AgentSessionView,
} from "@/features/agent-sessions/types";
import {
  AgentSessionPhaseChip,
  AgentSessionPrLink,
} from "@/features/agent-sessions/components/session-list";
import { EditTagsDialog } from "@/features/agent-sessions/components/edit-tags-dialog";

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
 * (branch when there is one, ticking duration, turns, model cost — ADR091 D6),
 * an inline draft-PR badge `#N`, the Open in Zed action (w2/m65), a "…"
 * overflow menu (open PR), and cancel-with-confirm. The duration ticks live
 * while the session is non-terminal; once terminal it pins to the session's
 * own end timestamp (via the mapper). Cancel is offered only while the session can
 * still be stopped and is disabled with a reason once canceling/canceled — the
 * confirm copy states pushed work is preserved. Beside it, while a turn is in
 * flight, Stop turn (ADR047 D12) ends only that turn and asks no confirmation.
 */
export function SessionDetailHeader({
  session,
  onChanged,
  backSearch,
}: SessionDetailHeaderProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { cancel, interrupt, deleteSession } = useAgentSessionMutations();
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
  const [stopping, setStopping] = useState(false);
  // The turn a stop was delivered for: the turn ends at the driver's next
  // poll and the Completer settles it within ~15 s, so the phase still reads
  // running after the 202 and Stop turn must not offer itself again.
  const [stoppedTurn, setStoppedTurn] = useState<number | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [tagsOpen, setTagsOpen] = useState(false);
  // Who started it (w7/m296): the roster label, or the raw subject for an API
  // key or a removed member. "" skips the roster read.
  const startedBy = useWorkspaceSubjectLabel(session.createdBy ?? "");

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

  // Stop turn (ADR047 D12) is offered while a turn is in flight and works once
  // its sandbox is up (the server refuses it as AGENT_SESSION_TURN_STARTING
  // before then). It ends only the turn: the session stays open, steerable and
  // keeps its queue, so unlike Cancel it asks for no confirmation.
  const canStopNow = cancelable && session.phase === "running";
  // A delivered stop rests until the turn is over: the phase is no longer
  // running, or a later turn (the next queued message's) has begun.
  const stopRequested =
    stoppedTurn !== null &&
    session.phase === "running" &&
    session.turns === stoppedTurn;
  const stopResting = stopping || stopRequested;
  const handleStop = useCallback(async () => {
    if (stopResting) return;
    setStopping(true);
    try {
      await interrupt(session.id);
      setStoppedTurn(session.turns);
      toast.success(t("agentSessions.stopTurnSuccess"));
    } catch (err) {
      toast.error(agentSessionErrorMessage(err, t));
    } finally {
      setStopping(false);
      // Whatever the answer, show what is true now: the turn is settling, or
      // had already ended on its own.
      void Promise.resolve(onChanged?.()).catch(() => undefined);
    }
  }, [interrupt, onChanged, session.id, session.turns, stopResting, t]);

  // Cmd/Ctrl+Shift+Backspace stops the running turn from anywhere on the page
  // (Devin's shortcut), without leaving the composer.
  useEffect(() => {
    if (!canStopNow) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key !== "Backspace" || !e.shiftKey || !(e.metaKey || e.ctrlKey)) {
        return;
      }
      e.preventDefault();
      if (!e.repeat) void handleStop();
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [canStopNow, handleStop]);

  async function handleConfirm() {
    setCanceling(true);
    try {
      await cancel(session.id);
      toast.success(t("agentSessions.cancelSuccess"));
      await onChanged?.();
    } catch (err) {
      toast.error(agentSessionErrorMessage(err, t));
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
        {/* One 16px line at every width, as the route skeleton draws it: on a
            narrow screen the metadata scrolls sideways instead of wrapping to
            a second line and shifting the page on load. */}
        <div className="text-muted-foreground mt-0.5 flex h-4 items-center gap-x-3 overflow-x-auto text-xs whitespace-nowrap [scrollbar-width:none] [&::-webkit-scrollbar]:hidden [&>*]:shrink-0">
          {/* A repo-less (chat-only) session has no branch — drop the whole
              element, icon included, rather than leaving an orphan GitBranch
              floating next to nothing (w1/m90 t002). */}
          {session.branch ? (
            <span className="inline-flex items-center gap-1 font-mono">
              <GitBranch className="size-3" />
              {session.branch}
            </span>
          ) : null}
          {/* The duration ticks off Date.now(), which the SSR pass and
              hydration read at different moments; the client clock replaces
              it on the next tick (w9/080, the RelativeAge convention). */}
          <span suppressHydrationWarning>
            {t("agentSessions.metaDuration", { duration })}
          </span>
          <span>{t("agentSessions.metaTurns", { count: session.turns })}</span>
          <SessionModelCost session={session} />
          {session.automationId ? (
            <StartedByAutomation
              automationId={session.automationId}
              runsAs={session.createdBy ? startedBy : null}
            />
          ) : session.createdBy ? (
            <span data-testid="started-by">
              {t("agentSessions.startedByPerson", { name: startedBy })}
            </span>
          ) : null}
          {session.tags.length > 0 ? (
            <span
              className="inline-flex items-center gap-1"
              data-testid="session-tags"
            >
              <Tag className="size-3" />
              {session.tags.join(", ")}
            </span>
          ) : null}
          <SessionSecrets session={session} />
          {session.isArchived ? (
            <Badge variant="secondary" className="gap-1">
              <Archive className="size-3" />
              {t("agentSessions.archivedBadge")}
            </Badge>
          ) : null}
        </div>
      </div>

      <HeaderPrBadge session={session} />

      <SessionFilesButton sessionId={session.id} />

      <OpenInZedButton session={session} />

      {cancelable ? (
        <StopTurnButton
          disabled={!canStopNow || stopResting}
          stopping={stopResting}
          reason={
            !canStopNow
              ? t("agentSessions.stopTurnStarting")
              : stopRequested
                ? t("agentSessions.stopTurnSuccess")
                : t("agentSessions.stopTurnHint")
          }
          onClick={() => void handleStop()}
        />
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
          {/* An archived session can be viewed, not modified (ADR065 D1). */}
          {session.isArchived ? null : (
            <DropdownMenuItem onClick={() => setTagsOpen(true)}>
              <Tag className="size-4" />
              {t("agentSessions.editTags")}
            </DropdownMenuItem>
          )}
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

      {tagsOpen ? (
        <EditTagsDialog
          sessionId={session.id}
          tags={session.tags}
          onOpenChange={setTagsOpen}
        />
      ) : null}

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

/**
 * The inline PR badge `#N` — links `prUrl` in a new tab when present — with
 * the PR's state (Open/Merged/Closed) beside it once bex has read it.
 */
function HeaderPrBadge({ session }: { session: AgentSessionView }) {
  if (session.prNumber == null || session.prNumber === 0) return null;
  return (
    <AgentSessionPrLink
      session={session}
      className="hidden shrink-0 sm:inline-flex"
      iconClassName="size-3.5"
    />
  );
}

/**
 * The Stop turn affordance (ADR047 D12): ends the running turn only. Its label
 * and icon differ from Cancel's, which ends the session. The tooltip names the
 * shortcut, or says why the button rests (the turn's sandbox is still starting,
 * a stop is in flight or delivered and the turn has not ended yet). Narrow screens keep the icon alone, still named for
 * assistive tech.
 */
function StopTurnButton({
  disabled,
  stopping,
  reason,
  onClick,
}: {
  disabled: boolean;
  stopping: boolean;
  reason: string;
  onClick: () => void;
}) {
  const { t } = useTranslations();
  const label = stopping
    ? t("agentSessions.stoppingTurn")
    : t("agentSessions.stopTurn");
  return (
    <Tooltip>
      {/* A disabled button swallows pointer events, so wrap it for the tooltip. */}
      <TooltipTrigger asChild>
        <span className="shrink-0" tabIndex={disabled ? 0 : undefined}>
          <Button
            size="sm"
            variant="secondary"
            disabled={disabled}
            onClick={onClick}
            aria-label={label}
            aria-keyshortcuts="Control+Shift+Backspace Meta+Shift+Backspace"
            className="shrink-0"
          >
            {stopping ? <Loader2 className="animate-spin" /> : <CircleStop />}
            <span className="hidden sm:inline">{label}</span>
          </Button>
        </span>
      </TooltipTrigger>
      <TooltipContent>{reason}</TooltipContent>
    </Tooltip>
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
        {/* eslint-disable-next-line jsx-a11y-x/no-noninteractive-tabindex -- the wrapper is the tooltip's trigger, focusable so keyboard users can reach the reason a disabled button explains */}
        <span className="shrink-0" tabIndex={0}>
          {button}
        </span>
      </TooltipTrigger>
      <TooltipContent>{reason}</TooltipContent>
    </Tooltip>
  );
}

/**
 * "Started by automation <name>" (ADR101 D9): the link from a session back to
 * the automation whose schedule fired it. The name is read on its own, so a
 * viewer who cannot read automations still sees the link, labelled by its id.
 */
function StartedByAutomation({
  automationId,
  runsAs,
}: {
  automationId: string;
  /** The member the automation ran as (w7/m296); null on older sessions. */
  runsAs: string | null;
}) {
  const { t } = useTranslations();
  const { automation } = useAgentAutomation(automationId);
  return (
    <span data-testid="started-by">
      {t("agentAutomations.startedBy")}{" "}
      <Link
        to="/automations/$automationId"
        params={{ automationId }}
        className="text-foreground underline underline-offset-2"
      >
        {automation?.name ?? automationId}
      </Link>
      {runsAs
        ? ` ${t("agentSessions.startedForPerson", { name: runsAs })}`
        : null}
    </span>
  );
}

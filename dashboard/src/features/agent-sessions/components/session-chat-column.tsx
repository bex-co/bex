import { useState, type ReactNode } from "react";
import { Loader2 } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { SessionDetailHeader } from "@/features/agent-sessions/components/session-detail-header";
import { FailureCallout } from "@/features/agent-sessions/components/failure-callout";
import { SteeringComposer } from "@/features/agent-sessions/components/steering-composer";
import { SessionConversation } from "@/features/agent-sessions/components/session-conversation";
import type { ConversationChatHandle } from "@/features/agent-sessions/components/session-conversation";
import type { AgentSessionView } from "@/features/agent-sessions/types";

export interface SessionChatColumnProps {
  session: AgentSessionView;
  chat: ConversationChatHandle | null;
  onChatStateChange: (handle: ConversationChatHandle | null) => void;
  onChanged: () => void;
}

/**
 * The agent-session chat column: header (top) + scrollable transcript + docked
 * composer (ADR047 D9, w3/m44). Two w2/m64 behaviors live here:
 *
 * - **Provisioning gate.** The conversation stream needs a sandbox to attach to.
 *   A brand-new session (or one whose sandbox failed to come up) has none yet, so
 *   the attach-ticket verb would 409 NOT_ATTACHABLE — mounting the stream there
 *   would render a spurious error. Until a sandbox id appears (via the header's
 *   phase polling) a placeholder is shown instead: a provisioning spinner while
 *   non-terminal, or just the failure/PR footer once terminal.
 * - **Optimistic redispatch echo.** The idle→redispatch steer path echoes while
 *   provisioning. It hides as soon as the durable turn count advances; gateway
 *   replay then projects the persisted prompt as a `data-user-prompt` part.
 */
export function SessionChatColumn({
  session,
  chat,
  onChatStateChange,
  onChanged,
}: SessionChatColumnProps) {
  const { t } = useTranslations();

  // Visibility is DERIVED, not cleared via an effect: the echo hides once the new
  // turn is recorded (turns advanced past the submit-time count), by which point
  // reconnect replay carries the durable prompt even while output is still live.
  const [pendingSteer, setPendingSteer] = useState<{
    prompt: string;
    atTurns: number;
  } | null>(null);
  const showPendingSteer =
    pendingSteer !== null &&
    !(session.turns > pendingSteer.atTurns && session.sandboxId);

  // Phase-derived terminus label ("went to sleep" / error / canceled) — only a
  // terminal session shows it (the impl gates on isTerminal + settled).
  const terminalLabel =
    session.phase === "completed"
      ? t("agentSessions.terminalStatus.completed")
      : session.phase === "failed"
        ? t("agentSessions.terminalStatus.failed")
        : session.phase === "canceled"
          ? t("agentSessions.terminalStatus.canceled")
          : undefined;

  // Inline transcript footer: the optimistic echo (if pending) and the failure
  // callout (with a Retry action, if the session failed) — both read as part of
  // the conversation flow rather than side cards. The draft-PR card that used to
  // close this footer was removed in w5/m65: a PR is now opt-in, and when there
  // is one the header's `#N` badge already links it.
  const footer = (
    <>
      {showPendingSteer ? (
        <OptimisticUserBubble text={pendingSteer.prompt} />
      ) : null}
      <FailureCallout session={session} onRetried={onChanged} />
    </>
  );

  // Attachable = a live sandbox to splice, OR a finished (terminal/hibernated)
  // session whose durable transcript replays through an ADR065 D2 replay-only
  // ticket. Only the genuinely unattachable window (provisioning, pre-dispatch)
  // shows the fallback — before m70 this gated on `sandboxId` alone, so every
  // reaped session rendered no conversation at all.
  const attachable = Boolean(session.sandboxId) || session.isFinished;
  const conversation = attachable ? (
    <SessionConversation
      // A durable turn is accepted before its replacement sandbox exists. Key
      // on both identities so each transition remounts useChat and replays the
      // newly persisted prompt, then attaches to the eventual fresh pod.
      key={`${session.id}:${session.turns}:${session.sandboxId}`}
      sessionId={session.id}
      isTerminal={session.isTerminal}
      onChatStateChange={onChatStateChange}
      footer={footer}
      terminalLabel={terminalLabel}
    />
  ) : (
    <ConversationFallback
      status={
        session.isTerminal ? null : (
          <div className="text-muted-foreground flex items-center gap-2 text-sm">
            <Loader2 aria-hidden className="size-4 shrink-0 animate-spin" />
            {t("agentSessions.provisioning")}
          </div>
        )
      }
      footer={footer}
    />
  );

  return (
    <>
      <SessionDetailHeader session={session} onChanged={onChanged} />

      <div className="min-h-0 flex-1">{conversation}</div>

      {/* An archived session refuses steer/resume (AGENT_SESSION_ARCHIVED), so
          the composer would be a dead input — the header's Unarchive is the way
          back (ADR065 D1). */}
      {session.isArchived ? null : (
        <SteeringComposer
          session={session}
          chat={chat}
          onSteered={onChanged}
          onOptimisticSteer={(prompt) =>
            setPendingSteer(prompt ? { prompt, atTurns: session.turns } : null)
          }
        />
      )}
    </>
  );
}

/**
 * A right-aligned user bubble matching the conversation's own user-turn styling,
 * used to optimistically echo a redispatch steer prompt in the transcript footer
 * before the durable transcript records the turn (w2/m64).
 */
function OptimisticUserBubble({ text }: { text: string }) {
  return (
    <div className="flex w-full justify-end pl-8">
      <div className="border-border/70 bg-muted text-foreground dark:bg-muted/80 max-w-[92%] rounded-xl rounded-br-md border px-3 py-1.5 text-sm leading-6 shadow-xs sm:max-w-md">
        {text}
      </div>
    </div>
  );
}

/**
 * The conversation area shown before a sandbox exists (w2/m64): the same
 * scroll-region shell the real column uses, carrying an optional status line
 * (the provisioning spinner) and the transcript footer (the failure callout) so
 * those still render while the stream is not yet attachable.
 */
function ConversationFallback({
  status,
  footer,
}: {
  status: ReactNode;
  footer: ReactNode;
}) {
  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="mx-auto w-full max-w-3xl space-y-2.5 px-4 py-3">
        {status}
        {footer}
      </div>
    </div>
  );
}

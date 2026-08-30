import { useState, type ReactNode } from "react";
import { Loader2 } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { SessionDetailHeader } from "@/features/agent-sessions/components/session-detail-header";
import { FailureCallout } from "@/features/agent-sessions/components/failure-callout";
import { SteeringComposer } from "@/features/agent-sessions/components/steering-composer";
import { SessionConversation } from "@/features/agent-sessions/components/session-conversation";
import type { ConversationChatHandle } from "@/features/agent-sessions/components/session-conversation";
import { ConversationSkeleton } from "@/features/agent-sessions/components/conversation-skeleton";
import type {
  AgentSessionListSearch,
  AgentSessionView,
} from "@/features/agent-sessions/types";
import {
  deriveConversationState,
  type ConversationState,
} from "@/features/agent-sessions/lib/conversation-state";

export interface SessionChatColumnProps {
  session: AgentSessionView;
  chat: ConversationChatHandle | null;
  onChatStateChange: (handle: ConversationChatHandle | null) => void;
  onChanged: () => void | Promise<unknown>;
  backSearch?: AgentSessionListSearch;
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
  backSearch,
}: SessionChatColumnProps) {
  const { t } = useTranslations();
  const [transportState, setTransportState] =
    useState<ConversationState>("not-started");

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

  // Attachable = a live sandbox to splice, a finished (terminal/hibernated)
  // session replaying through an ADR065 D2 replay-only ticket, or one with an
  // established transcript once provisioning has settled. While a non-terminal
  // session is creating, redispatching, or resuming without a sandbox, keep the
  // provisioning fallback so attach-ticket failures never surface as a false
  // "stream unavailable" error (w5/m78 t003).
  const provisioningWithoutSandbox =
    !session.sandboxId &&
    !session.isFinished &&
    (session.phase === "creating" ||
      session.phase === "redispatching" ||
      session.phase === "resuming");
  const attachable =
    Boolean(session.sandboxId) ||
    session.isFinished ||
    (session.turns >= 2 && !provisioningWithoutSandbox);
  const conversationState = attachable
    ? transportState
    : deriveConversationState({
        phase: session.phase,
        isTerminal: session.isTerminal,
      });
  const conversation = attachable ? (
    <SessionConversation
      // Keyed ONLY on the session id (not turns/sandbox), so the `useChat`
      // instance is stable across the session's lifecycle — no remount on a
      // follow-up/sandbox-swap/settle. Those transitions ride `attachSignal`
      // instead, driving an in-place re-attach (see the impl prop).
      key={session.id}
      sessionId={session.id}
      isTerminal={session.isTerminal}
      phase={session.phase}
      attachSignal={`${session.turns}:${session.sandboxId}:${session.isTerminal}`}
      onChatStateChange={onChatStateChange}
      onConversationStateChange={setTransportState}
      footer={footer}
      terminalLabel={terminalLabel}
    />
  ) : (
    <ConversationFallback
      status={
        session.isTerminal ? null : (
          <div
            role="status"
            className="text-muted-foreground flex items-center gap-2 text-sm"
          >
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
      <SessionDetailHeader
        session={session}
        onChanged={onChanged}
        backSearch={backSearch}
      />

      <div className="min-h-0 flex-1">{conversation}</div>

      {/* An archived session refuses steer/resume (AGENT_SESSION_ARCHIVED), so
          the composer would be a dead input — the header's Unarchive is the way
          back (ADR065 D1). */}
      {session.isArchived ? null : (
        <SteeringComposer
          session={session}
          chat={chat}
          conversationState={conversationState}
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
        {status && (
          <>
            {status}
            <ConversationSkeleton />
          </>
        )}
        {footer}
      </div>
    </div>
  );
}

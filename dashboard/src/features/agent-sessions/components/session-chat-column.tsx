import { useState, type ReactNode } from "react";
import { Loader2 } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { SessionDetailHeader } from "@/features/agent-sessions/components/session-detail-header";
import { PrCard } from "@/features/agent-sessions/components/pr-card";
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
  onOpenEvidence: () => void;
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
 * - **Optimistic redispatch echo.** The idle→redispatch steer path has no
 *   `useChat` optimism, so the composer hands the prompt up here to echo it in the
 *   transcript immediately; it hides once the new turn is recorded AND settled
 *   (the durable transcript then carries it), or on a synchronous rejection.
 */
export function SessionChatColumn({
  session,
  chat,
  onChatStateChange,
  onChanged,
  onOpenEvidence,
}: SessionChatColumnProps) {
  const { t } = useTranslations();

  // Visibility is DERIVED, not cleared via an effect: the echo hides once the new
  // turn is both recorded (turns advanced past the submit-time count) and settled
  // (terminal), by which point the durable transcript carries the turn.
  const [pendingSteer, setPendingSteer] = useState<{
    prompt: string;
    atTurns: number;
  } | null>(null);
  const showPendingSteer =
    pendingSteer !== null &&
    !(session.isTerminal && session.turns > pendingSteer.atTurns);

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

  // Inline transcript footer: the optimistic echo (if pending), the failure
  // callout (with a Retry action, if the session failed), then the draft-PR card
  // — all read as part of the conversation flow rather than side cards.
  const footer = (
    <>
      {showPendingSteer ? (
        <OptimisticUserBubble text={pendingSteer.prompt} />
      ) : null}
      <FailureCallout session={session} onRetried={onChanged} />
      <PrCard session={session} />
    </>
  );

  const conversation = session.sandboxId ? (
    <SessionConversation
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
      <SessionDetailHeader
        session={session}
        onCanceled={onChanged}
        onOpenEvidence={onOpenEvidence}
      />

      <div className="min-h-0 flex-1">{conversation}</div>

      <SteeringComposer
        session={session}
        chat={chat}
        onSteered={onChanged}
        onOptimisticSteer={(prompt) =>
          setPendingSteer(prompt ? { prompt, atTurns: session.turns } : null)
        }
      />
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
 * (the provisioning spinner) and the transcript footer (failure callout + PR
 * card) so those still render while the stream is not yet attachable.
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

import { useId, useState } from "react";
import { MessageCircleQuestion, ShieldAlert } from "lucide-react";
import { toast } from "sonner";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Textarea } from "@/common/components/ui/textarea";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useAgentSessionMutations,
  type DecisionAnswer,
} from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import {
  AgentSessionError,
  agentSessionErrorMessage,
} from "@/features/agent-sessions/lib/errors";
import {
  isAgentSessionBlocked,
  type AgentSessionDecisionView,
  type AgentSessionView,
} from "@/features/agent-sessions/types";

/** Refusals that mean the decision is no longer open: re-read and say why. */
const SETTLED_CODES: Record<string, string> = {
  AGENT_SESSION_DECISION_ANSWERED: "agentSessions.decision.alreadyAnswered",
  AGENT_SESSION_DECISION_EXPIRED: "agentSessions.decision.expired",
  AGENT_SESSION_DECISION_NOT_FOUND: "agentSessions.decision.expired",
};

export interface DecisionCalloutProps {
  session: AgentSessionView;
  /** Re-read the session once an answer lands (or the decision moved on). */
  onAnswered: () => void | Promise<unknown>;
}

/**
 * The paused turn's open decision (ADR100 D6), docked above the composer so it
 * stays in view however long the transcript is: Devin's waiting_for_approval /
 * waiting_for_user. An approval shows the gated command, the paths it touches
 * and the matched rule, with Allow once / Allow for this session / Deny; a
 * question shows its text, any suggested answers, and a reply box. Renders
 * nothing unless the session is paused on a still-pending decision.
 */
export function DecisionCallout({ session, onAnswered }: DecisionCalloutProps) {
  const decision = session.outcome?.pendingDecision ?? null;
  if (!isAgentSessionBlocked(session) || decision?.state !== "pending") {
    return null;
  }
  // Keyed by decision so a reply typed for one never carries to the next.
  return (
    <DecisionCalloutBody
      key={decision.id}
      sessionId={session.id}
      decision={decision}
      onAnswered={onAnswered}
    />
  );
}

function DecisionCalloutBody({
  sessionId,
  decision,
  onAnswered,
}: {
  sessionId: string;
  decision: AgentSessionDecisionView;
  onAnswered: () => void | Promise<unknown>;
}) {
  const { t } = useTranslations();
  const { answerDecision } = useAgentSessionMutations();
  const replyId = useId();
  const [sending, setSending] = useState(false);
  const [reply, setReply] = useState("");
  const [inlineError, setInlineError] = useState<string | null>(null);
  const question = decision.kind === "question";
  const { detail } = decision;

  async function send(answer: DecisionAnswer) {
    setSending(true);
    setInlineError(null);
    try {
      await answerDecision(sessionId, decision.id, answer);
      await onAnswered();
    } catch (err) {
      const settled =
        err instanceof AgentSessionError ? SETTLED_CODES[err.code] : undefined;
      if (settled) {
        toast.info(t(settled));
        await onAnswered();
      } else {
        setInlineError(agentSessionErrorMessage(err, t));
      }
    } finally {
      setSending(false);
    }
  }

  const trimmed = reply.trim();
  return (
    <Alert
      className="border-amber-500/40 bg-amber-500/5"
      data-testid="agent-decision-callout"
    >
      {question ? <MessageCircleQuestion /> : <ShieldAlert />}
      <AlertTitle>
        {question
          ? t("agentSessions.decision.questionTitle")
          : t("agentSessions.decision.approvalTitle")}
      </AlertTitle>
      <AlertDescription className="flex min-w-0 flex-col items-stretch gap-2">
        {question ? (
          <p className="text-foreground whitespace-pre-wrap">
            {detail.question ?? decision.title}
          </p>
        ) : (
          <>
            <p className="text-foreground">{decision.title}</p>
            {detail.command ? (
              <pre className="bg-muted max-h-40 overflow-auto rounded-md px-2 py-1.5 font-mono text-xs whitespace-pre-wrap break-all">
                {detail.command}
              </pre>
            ) : null}
            {detail.paths?.length ? (
              <ul
                className="font-mono text-xs break-all"
                aria-label={t("agentSessions.decision.pathsLabel")}
              >
                {detail.paths.map((path) => (
                  <li key={path}>{path}</li>
                ))}
              </ul>
            ) : null}
            {detail.rule ? (
              <p className="text-xs">
                {t("agentSessions.decision.rule", { rule: detail.rule })}
              </p>
            ) : null}
          </>
        )}

        {question ? (
          <>
            {detail.options?.length ? (
              <div
                className="flex flex-wrap gap-2"
                role="group"
                aria-label={t("agentSessions.decision.optionsLabel")}
              >
                {detail.options.map((option) => (
                  <Button
                    key={option}
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={sending}
                    onClick={() => void send({ text: option })}
                  >
                    {option}
                  </Button>
                ))}
              </div>
            ) : null}
            <form
              className="flex flex-col gap-2 sm:flex-row sm:items-end"
              onSubmit={(event) => {
                event.preventDefault();
                if (trimmed) void send({ text: trimmed });
              }}
            >
              <Textarea
                id={replyId}
                value={reply}
                onChange={(event) => setReply(event.target.value)}
                onKeyDown={(event) => {
                  if (
                    event.key === "Enter" &&
                    !event.shiftKey &&
                    !event.nativeEvent.isComposing
                  ) {
                    event.preventDefault();
                    if (trimmed && !sending) void send({ text: trimmed });
                  }
                }}
                rows={2}
                disabled={sending}
                aria-label={t("agentSessions.decision.replyLabel")}
                placeholder={t("agentSessions.decision.replyPlaceholder")}
                className="min-h-16 flex-1 bg-background"
              />
              <Button type="submit" size="sm" disabled={sending || !trimmed}>
                {t("agentSessions.decision.reply")}
              </Button>
            </form>
          </>
        ) : (
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              size="sm"
              disabled={sending}
              onClick={() => void send({ allow: true, scope: "once" })}
            >
              {t("agentSessions.decision.allowOnce")}
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={sending}
              onClick={() => void send({ allow: true, scope: "session" })}
            >
              {t("agentSessions.decision.allowSession")}
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="text-destructive"
              disabled={sending}
              onClick={() => void send({ allow: false })}
            >
              {t("agentSessions.decision.deny")}
            </Button>
          </div>
        )}

        <p className="text-xs">
          {question
            ? t("agentSessions.decision.questionExpiry")
            : t("agentSessions.decision.approvalExpiry")}
        </p>
        {inlineError ? (
          <p
            className="text-destructive text-sm"
            role="alert"
            data-testid="agent-decision-error"
          >
            {inlineError}
          </p>
        ) : null}
      </AlertDescription>
    </Alert>
  );
}

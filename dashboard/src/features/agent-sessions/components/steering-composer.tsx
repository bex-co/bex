import { useState } from "react";
import { Loader2, SendHorizonal } from "lucide-react";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Textarea } from "@/common/components/ui/textarea";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import {
  AgentSessionError,
  AgentSessionsUnavailableError,
} from "@/features/agent-sessions/lib/errors";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import type { ConversationChatHandle } from "@/features/agent-sessions/components/session-conversation";

/** The routed target of a steering message (ADR047 D9). */
type SteerRoute = "chat" | "redispatch";

export interface SteeringComposerProps {
  session: AgentSessionView;
  /**
   * The live conversation's `useChat` handle, lifted from the column. Present
   * only while the stream is healthy; `null` disables the live (chat POST) path.
   */
  chat: ConversationChatHandle | null;
  /** Re-read the session after a redispatch converges (turns/phase change). */
  onSteered?: () => void;
}

/**
 * The single, state-routed steering composer (ADR047 § D9). One textarea, two
 * destinations decided by the session's phase:
 *
 * - a **live** (running/creating/resuming/redispatching) session → the message
 *   is sent as a chat `POST` through the conversation column's own `useChat`
 *   (`chat.sendMessage`), so it appears in the transcript on the same branch;
 * - an **idle** (completed/failed) session → the message redispatches a new
 *   turn via t001's `steer` mutation, and the header refetch surfaces the
 *   incremented `turns`.
 *
 * It is always rendered but disabled with a stated reason while the session is
 * `canceling`/`canceled`, while a turn is in flight, or when the live stream is
 * unavailable (so the live path can't POST into a dead stream).
 */
export function SteeringComposer({
  session,
  chat,
  onSteered,
}: SteeringComposerProps) {
  const { t } = useTranslations();
  const { steer } = useAgentSessionMutations();
  const [value, setValue] = useState("");
  const [pending, setPending] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const isCanceling = session.phase === "canceling";
  const isCanceled = session.phase === "canceled";
  const turnInFlight =
    chat?.status === "streaming" || chat?.status === "submitted";

  // Route: idle (completed/failed) → redispatch mutation; otherwise the live
  // stream POST. Canceling/canceled fall through as fully disabled below.
  const route: SteerRoute = session.isSteerable ? "redispatch" : "chat";
  const liveStreamMissing =
    route === "chat" && !isCanceling && !isCanceled && !chat;

  // A single reason string when the composer is hard-disabled; null when it's
  // ready to accept input.
  const disabledReason: string | null = isCanceling
    ? t("agentSessions.steerDisabledCanceling")
    : isCanceled
      ? t("agentSessions.steerDisabledCanceled")
      : liveStreamMissing
        ? t("agentSessions.steerDisabledStream")
        : turnInFlight
          ? t("agentSessions.steerDisabledInFlight")
          : null;

  const hardDisabled = isCanceling || isCanceled || liveStreamMissing;
  const busy = pending || turnInFlight;
  const inputDisabled = hardDisabled;
  const submitDisabled = hardDisabled || busy || value.trim().length === 0;

  function handleError(err: unknown) {
    if (err instanceof AgentSessionsUnavailableError) {
      setSubmitError(t("agentSessions.unavailableBody"));
      return;
    }
    if (err instanceof AgentSessionError) {
      setSubmitError(
        t(err.messageKey, { ...err.params, defaultValue: err.message }),
      );
      return;
    }
    setSubmitError(err instanceof Error ? err.message : String(err));
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    const prompt = value.trim();
    if (!prompt || hardDisabled || busy) return;
    setSubmitError(null);
    setPending(true);
    try {
      if (route === "redispatch") {
        await steer(session.id, prompt);
        toast.success(t("agentSessions.steerSuccess"));
        onSteered?.();
      } else {
        // Live path: append the turn to the conversation's own useChat stream.
        if (!chat) return;
        await chat.sendMessage(prompt);
      }
      setValue("");
    } catch (err) {
      handleError(err);
    } finally {
      setPending(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">
          {t("agentSessions.steerTitle")}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {submitError ? (
          <Alert variant="destructive">
            <AlertTitle>{t("agentSessions.steerErrorTitle")}</AlertTitle>
            <AlertDescription>{submitError}</AlertDescription>
          </Alert>
        ) : null}

        <form onSubmit={(e) => void onSubmit(e)} className="space-y-2">
          <Textarea
            value={value}
            onChange={(e) => setValue(e.target.value)}
            rows={3}
            disabled={inputDisabled}
            placeholder={
              route === "redispatch"
                ? t("agentSessions.steerPlaceholderIdle")
                : t("agentSessions.steerPlaceholderLive")
            }
          />
          <div className="flex items-center justify-between gap-3">
            <p className="text-muted-foreground text-xs">
              {disabledReason ??
                (route === "redispatch"
                  ? t("agentSessions.steerHintIdle")
                  : t("agentSessions.steerHintLive"))}
            </p>
            <Button type="submit" size="sm" disabled={submitDisabled}>
              {busy ? (
                <>
                  <Loader2 className="animate-spin" />
                  {t("agentSessions.steerSending")}
                </>
              ) : (
                <>
                  <SendHorizonal />
                  {t("agentSessions.steerSubmit")}
                </>
              )}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}

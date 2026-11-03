import { useState } from "react";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";
import { Loader2, SendHorizonal } from "lucide-react";
import { toast } from "sonner";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Textarea } from "@/common/components/ui/textarea";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import { useAgentSessionCapabilities } from "@/features/agent-sessions/hooks/use-agent-session-capabilities";
import { useAttachmentUploads } from "@/features/agent-sessions/hooks/use-attachment-uploads";
import {
  AttachButton,
  AttachmentChips,
  DropOverlay,
} from "@/features/agent-sessions/components/attachment-chips";
import { useAttachmentDropTarget } from "@/features/agent-sessions/hooks/use-attachment-drop-target";
import { agentSessionErrorMessage } from "@/features/agent-sessions/lib/errors";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import type { ConversationChatHandle } from "@/features/agent-sessions/components/session-conversation";
import type { ConversationState } from "@/features/agent-sessions/lib/conversation-state";

/** The routed target of a steering message (ADR047 D9). */
type SteerRoute = "chat" | "redispatch";

export interface SteeringComposerProps {
  session: AgentSessionView;
  /**
   * The live conversation's `useChat` handle, lifted from the column. Present
   * only while the stream is healthy; `null` disables the live (chat POST) path.
   */
  chat: ConversationChatHandle | null;
  conversationState?: ConversationState;
  /** Re-read the session after a redispatch converges (turns/phase change). */
  onSteered?: () => void;
  /**
   * Optimistic echo for the **redispatch** path (w2/m64): called with the prompt
   * the instant a redispatch steer is submitted so the detail page can show the
   * message immediately (the idle path has no `useChat` optimism of its own),
   * and with `null` to roll it back if the steer is rejected synchronously. The
   * The disabled live path never calls this callback.
   */
  onOptimisticSteer?: (prompt: string | null) => void;
}

/**
 * The bottom-docked chat composer (ADR047 § D9, w3/m44). One auto-growing
 * textarea (Enter-to-send, Shift+Enter for a newline) with two destinations
 * decided by the session's phase:
 *
 * - a **live** (running/creating/resuming/redispatching) session → disabled until
 *   the current turn settles, because public live POST cannot atomically persist
 *   its prompt with control-plane turn state;
 * - an **idle** (completed/failed) session → the message redispatches a new
 *   turn via t001's `steer` mutation, and the header refetch surfaces the
 *   incremented `turns`.
 *
 * It is always rendered but disabled with a stated reason while the session is
 * `canceling`/`canceled` or while a turn is in flight.
 */
export function SteeringComposer({
  session,
  chat,
  conversationState,
  onSteered,
  onOptimisticSteer,
}: SteeringComposerProps) {
  const { t } = useTranslations();
  const { steer } = useAgentSessionMutations();
  // Files belong to the session's own workspace, which may not be the one
  // selected in the switcher; so do its limits.
  const { capabilities } = useAgentSessionCapabilities(session.ownerId);
  const limits = capabilities?.attachments ?? null;
  const uploads = useAttachmentUploads(limits, session.ownerId);
  const [value, setValue] = useState("");
  const [pending, setPending] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const isCanceling = session.phase === "canceling";
  const isCanceled = session.phase === "canceled";
  const turnInFlight =
    chat?.status === "streaming" || chat?.status === "submitted";

  // Route: idle (completed/failed) → durable redispatch mutation; every live
  // phase stays disabled until it reaches one of those terminal states.
  const route: SteerRoute = session.isSteerable ? "redispatch" : "chat";
  // w5/m71 closes the gateway POST path because it could execute a prompt
  // without atomically recording the turn. Follow-ups are accepted only after
  // the current control-plane turn settles, through the durable Steer mutation.
  const liveTurnUnavailable = route === "chat" && !isCanceling && !isCanceled;

  // A single reason string when the composer is hard-disabled; null when it's
  // ready to accept input.
  const disabledReason: string | null = isCanceling
    ? t("agentSessions.steerDisabledCanceling")
    : isCanceled
      ? t("agentSessions.steerDisabledCanceled")
      : conversationState === "broken"
        ? t("agentSessions.steerDisabledStream")
        : liveTurnUnavailable
          ? t("agentSessions.steerDisabledWaitForTurn")
          : turnInFlight
            ? t("agentSessions.steerDisabledInFlight")
            : null;

  const hardDisabled = isCanceling || isCanceled || liveTurnUnavailable;
  const busy = pending || turnInFlight;
  // Attachments ride only the durable redispatch path (the live path is off),
  // and not while a send is in flight: a file added then would miss it.
  const canAttach = Boolean(limits?.enabled) && !hardDisabled && !pending;
  const attachmentsBlock = uploads.uploading || uploads.hasErrors;
  const submitDisabled =
    hardDisabled || busy || attachmentsBlock || value.trim().length === 0;
  const drop = useAttachmentDropTarget(uploads.add, canAttach);

  const attachmentHint = uploads.hasErrors
    ? t("agentSessions.attachmentsBlockSend")
    : uploads.uploading
      ? t("agentSessions.attachmentsUploading")
      : null;
  const routeHint =
    route === "redispatch"
      ? t("agentSessions.steerHintIdle")
      : t("agentSessions.steerHintLive");

  function handleError(err: unknown) {
    setSubmitError(agentSessionErrorMessage(err, t));
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    const prompt = value.trim();
    if (!prompt || hardDisabled || busy || attachmentsBlock) return;
    setSubmitError(null);
    setPending(true);
    try {
      if (route === "redispatch") {
        // Optimistically echo the prompt now — the redispatch runs fast then
        // provisions in the background (w2/m64), so the message must appear
        // immediately rather than after the round trip.
        onOptimisticSteer?.(prompt);
        const sent = uploads.readyIds;
        await steer(session.id, prompt, { attachmentIds: sent });
        uploads.clear(sent);
        toast.success(t("agentSessions.steerSuccess"));
        onSteered?.();
      } else {
        // Defensive dead branch while liveTurnUnavailable is true. Keep the
        // handle seam for a future transactional live-turn protocol.
        if (!chat) return;
        await chat.sendMessage(prompt);
      }
      setValue("");
    } catch (err) {
      // A synchronous rejection (conflict/unavailable) rolls the optimistic
      // echo back so a rejected message never lingers in the transcript.
      if (route === "redispatch") onOptimisticSteer?.(null);
      // Closing the ADR075 D7 payment dialog is a user choice, not a failure.
      if (!isPaymentOnboardingCancelled(err)) handleError(err);
    } finally {
      setPending(false);
    }
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    // Enter sends; Shift+Enter inserts a newline (the chat-composer convention).
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      if (!submitDisabled) void onSubmit(e);
    }
  }

  return (
    <div className="bg-background shrink-0">
      <div className="mx-auto w-full max-w-3xl space-y-1.5 px-4 py-2">
        {submitError ? (
          <Alert variant="destructive">
            <AlertTitle>{t("agentSessions.steerErrorTitle")}</AlertTitle>
            <AlertDescription>{submitError}</AlertDescription>
          </Alert>
        ) : null}

        <form onSubmit={(e) => void onSubmit(e)}>
          <div
            {...drop.handlers}
            className={cn(
              "border-input bg-background focus-within:border-ring focus-within:ring-ring/40 relative rounded-xl border shadow-xs transition-[color,box-shadow] focus-within:ring-[3px]",
              hardDisabled && "opacity-70",
              drop.dragging && "border-ring ring-ring/40 ring-[3px]",
            )}
          >
            <DropOverlay visible={drop.dragging} />
            <AttachmentChips uploads={uploads} limits={limits} />
            <div className="flex items-end gap-2 px-2.5 py-1.5">
              {canAttach ? (
                <AttachButton onFiles={uploads.add} disabled={pending} />
              ) : null}
              {/* `field-sizing-content` (base Textarea) auto-grows the input to
                fit; the max-height + overflow bound it for a long follow-up. */}
              <Textarea
                value={value}
                onChange={(e) => setValue(e.target.value)}
                onKeyDown={handleKeyDown}
                rows={1}
                disabled={hardDisabled}
                placeholder={
                  route === "redispatch"
                    ? t("agentSessions.steerPlaceholderIdle")
                    : t("agentSessions.steerPlaceholderLive")
                }
                className="max-h-[200px] min-h-0 resize-none overflow-y-auto border-0 bg-transparent p-0 py-1.5 shadow-none focus-visible:ring-0 dark:bg-transparent"
              />
              <Button
                type="submit"
                size="sm"
                disabled={submitDisabled}
                className="shrink-0 rounded-xl"
              >
                {pending ? (
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
          </div>
          <p className="text-muted-foreground mt-1 px-1 text-xs">
            {disabledReason ?? attachmentHint ?? routeHint}
          </p>
        </form>
      </div>
    </div>
  );
}

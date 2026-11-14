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
import { useIdempotencyKey } from "@/features/agent-sessions/hooks/use-idempotency-key";
import {
  AttachButton,
  AttachmentChips,
  DropOverlay,
} from "@/features/agent-sessions/components/attachment-chips";
import { useAttachmentDropTarget } from "@/features/agent-sessions/hooks/use-attachment-drop-target";
import {
  ComposerSkillChips,
  ComposerSkills,
} from "@/features/agent-sessions/components/composer-skills";
import {
  composerSkillsFingerprint,
  composerSkillsInput,
  composerSkillsValid,
  type ComposerSkill,
} from "@/features/agent-sessions/lib/composer-skills";
import { agentSessionErrorMessage } from "@/features/agent-sessions/lib/errors";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import type { ConversationChatHandle } from "@/features/agent-sessions/components/session-conversation";
import type { ConversationState } from "@/features/agent-sessions/lib/conversation-state";

export interface SteeringComposerProps {
  session: AgentSessionView;
  /**
   * The live conversation's `useChat` handle, lifted from the column. The
   * composer no longer sends through it: a message to a session whose turn
   * runs is queued (ADR047 D12), and public live injection stays unavailable
   * (D9). Kept as the seam for a future transactional live-turn protocol.
   */
  chat?: ConversationChatHandle | null;
  conversationState?: ConversationState;
  /** Re-read the session after a send (a turn started, or a message queued). */
  onSteered?: () => void;
  /**
   * Optimistic echo for a send that starts a turn (w2/m64): called with the
   * prompt the instant it is submitted so the detail page can show the message
   * immediately (the idle path has no `useChat` optimism of its own), and with
   * `null` to take it back if the steer is rejected, or if the server queued
   * the message instead (a queued message shows as a chip, not as a turn). A
   * send the composer expects to queue never calls it.
   */
  onOptimisticSteer?: (prompt: string | null) => void;
  /** How many messages already wait in the session's queue (ADR047 D12). */
  queuedCount?: number;
}

/**
 * The bottom-docked chat composer (ADR047 § D9, w3/m44; D12). One auto-growing
 * textarea (Enter-to-send, Shift+Enter for a newline). Every message goes
 * through the `steer` mutation, which the session's state routes:
 *
 * - an **idle** (completed/failed) session with nothing queued starts a new
 *   turn, and the header refetch surfaces the incremented `turns`;
 * - a session whose turn is **running** (creating/running/resuming/
 *   redispatching), or one with messages already waiting, **queues** the
 *   message. It appears at once in the queue list above the composer and runs
 *   as its own turn when the current one settles.
 *
 * It stays enabled in every state that can take a message: only a canceling or
 * canceled session disables it, with a stated reason (an archived session
 * never mounts it). `steerable` is not the condition, since it is true only
 * for an idle session.
 */
export function SteeringComposer({
  session,
  conversationState,
  onSteered,
  onOptimisticSteer,
  queuedCount = 0,
}: SteeringComposerProps) {
  const { t } = useTranslations();
  const { steer } = useAgentSessionMutations();
  // Files belong to the session's own workspace, which may not be the one
  // selected in the switcher; so do its limits.
  const { capabilities } = useAgentSessionCapabilities(session.ownerId);
  const limits = capabilities?.attachments ?? null;
  const uploads = useAttachmentUploads(limits, session.ownerId);
  const [value, setValue] = useState("");
  // Workspace skills the follow-up turn must follow (ADR099 D4).
  const [skills, setSkills] = useState<ComposerSkill[]>([]);
  const [pending, setPending] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const isCanceling = session.phase === "canceling";
  const isCanceled = session.phase === "canceled";
  const hardDisabled = isCanceling || isCanceled;
  // A send queues while a turn runs, and behind messages that already wait (the
  // server queues it then too, so a direct send cannot jump the queue).
  const willQueue = !session.isSteerable || queuedCount > 0;
  // One key per distinct queued send: a retry of the same message after a
  // failure reuses it (the server hands back the message it already queued),
  // an edited message gets a fresh one.
  const submitKey = useIdempotencyKey();

  // A single reason string when the composer is hard-disabled; null when it's
  // ready to accept input.
  const disabledReason: string | null = isCanceling
    ? t("agentSessions.steerDisabledCanceling")
    : isCanceled
      ? t("agentSessions.steerDisabledCanceled")
      : conversationState === "broken"
        ? t("agentSessions.steerDisabledStream")
        : null;

  // A file added while a send is in flight would miss it.
  const canAttach = Boolean(limits?.enabled) && !hardDisabled && !pending;
  const attachmentsBlock = uploads.uploading || uploads.hasErrors;
  const skillsInvalid = !composerSkillsValid(skills);
  const submitDisabled =
    hardDisabled ||
    pending ||
    attachmentsBlock ||
    skillsInvalid ||
    value.trim().length === 0;
  const drop = useAttachmentDropTarget(uploads.add, canAttach);

  const attachmentHint = uploads.hasErrors
    ? t("agentSessions.attachmentsBlockSend")
    : uploads.uploading
      ? t("agentSessions.attachmentsUploading")
      : null;
  const routeHint = willQueue
    ? t("agentSessions.steerHintQueue")
    : t("agentSessions.steerHintIdle");

  function handleError(err: unknown) {
    setSubmitError(agentSessionErrorMessage(err, t));
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    const prompt = value.trim();
    if (!prompt || hardDisabled || pending || attachmentsBlock || skillsInvalid)
      return;
    setSubmitError(null);
    setPending(true);
    const queues = willQueue;
    let echoed = false;
    try {
      // Optimistically echo a send that starts a turn — it runs fast then
      // provisions in the background (w2/m64), so the message must appear
      // immediately rather than after the round trip. A queued one is a chip.
      if (!queues) {
        onOptimisticSteer?.(prompt);
        echoed = true;
      }
      const sent = uploads.readyIds;
      let idempotencyKey: string | undefined;
      if (queues) {
        idempotencyKey = submitKey.keyFor(
          JSON.stringify([
            session.id,
            prompt,
            sent,
            composerSkillsFingerprint(skills),
          ]),
        );
      }
      const result = await steer(session.id, prompt, {
        attachmentIds: sent,
        ...(skills.length > 0 ? { skills: composerSkillsInput(skills) } : {}),
        ...(idempotencyKey ? { idempotencyKey } : {}),
      });
      submitKey.release();
      uploads.clear(sent);
      setSkills([]);
      if (result.queuedMessage) {
        // The server queued it (the session was busy after all): no turn began.
        if (echoed) onOptimisticSteer?.(null);
        toast.success(t("agentSessions.steerQueued"));
      } else {
        toast.success(t("agentSessions.steerSuccess"));
      }
      onSteered?.();
      setValue("");
    } catch (err) {
      // A synchronous rejection (conflict/unavailable) rolls the optimistic
      // echo back so a rejected message never lingers in the transcript.
      if (echoed) onOptimisticSteer?.(null);
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
            <ComposerSkillChips
              value={skills}
              onChange={setSkills}
              disabled={pending}
            />
            <div className="flex items-end gap-2 px-2.5 py-1.5">
              {canAttach ? (
                <AttachButton onFiles={uploads.add} disabled={pending} />
              ) : null}
              {!hardDisabled ? (
                <ComposerSkills
                  ownerId={session.ownerId}
                  value={skills}
                  onChange={setSkills}
                  disabled={pending}
                />
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
                  willQueue
                    ? t("agentSessions.steerPlaceholderQueue")
                    : t("agentSessions.steerPlaceholderIdle")
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

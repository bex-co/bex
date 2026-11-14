import { useState } from "react";
import {
  AlertCircle,
  ArrowDown,
  ArrowUp,
  Loader2,
  Paperclip,
  Pencil,
  X,
  Zap,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import { Textarea } from "@/common/components/ui/textarea";
import { useTranslations } from "@/common/hooks/use-translations";
import { useQueuedMessageMutations } from "@/features/agent-sessions/hooks/use-queued-message-mutations";
import { agentSessionErrorMessage } from "@/features/agent-sessions/lib/errors";
import type {
  AgentSessionView,
  QueuedMessageView,
} from "@/features/agent-sessions/types";

/** How much of a long prompt a row shows; editing it shows all of it. */
const PREVIEW_MAX_CHARS = 600;

export interface QueuedMessageListProps {
  session: AgentSessionView;
  /** The session's waiting messages, next to run first. */
  messages: QueuedMessageView[];
  /** Re-read the session after "Send now", which stops the running turn. */
  onChanged?: () => void | Promise<unknown>;
}

/**
 * The messages waiting behind a session's running turn (ADR047 D12, Devin's
 * queue): one row each, with its prompt, the reason the dispatcher could not
 * start it yet when there is one, and the ways to change the order of events.
 * Edit in place (Enter saves, Cmd/Ctrl+Enter saves and sends it now, Esc
 * discards), remove, move up or down, and "Send now", which stops the running
 * turn and moves the message to the front. With nothing queued it renders
 * nothing at all, so a session without a queue keeps its page as it was.
 */
export function QueuedMessageList(props: QueuedMessageListProps) {
  if (props.messages.length === 0) return null;
  return <QueuedMessageRows {...props} />;
}

function QueuedMessageRows({
  session,
  messages,
  onChanged,
}: QueuedMessageListProps) {
  const { t } = useTranslations();
  const { edit, remove, move, sendNow } = useQueuedMessageMutations(session.id);
  // One message changes at a time: its row's controls rest until it settles.
  const [busyId, setBusyId] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);

  async function run(messageId: string, action: () => Promise<void>) {
    setBusyId(messageId);
    setError(null);
    try {
      await action();
    } catch (err) {
      setError(agentSessionErrorMessage(err, t));
    } finally {
      setBusyId(null);
    }
  }

  async function runSendNow(messageId: string, index: number) {
    const { stopped } = await sendNow(messageId, index);
    toast.success(
      t(
        stopped
          ? "agentSessions.queueSendNowStopping"
          : "agentSessions.queueSendNowNext",
      ),
    );
    await onChanged?.();
  }

  function startEdit(message: QueuedMessageView) {
    setError(null);
    setDraft(message.prompt);
    setEditingId(message.id);
  }

  function stopEdit() {
    setEditingId(null);
    setDraft("");
  }

  /** Saves the draft when it changed, then optionally sends the message now. */
  function saveEdit(
    message: QueuedMessageView,
    index: number,
    thenSendNow: boolean,
  ) {
    const prompt = draft.trim();
    if (!prompt) return;
    void run(message.id, async () => {
      if (prompt !== message.prompt) await edit(message.id, prompt);
      stopEdit();
      if (thenSendNow) await runSendNow(message.id, index);
    });
  }

  function handleEditKeyDown(
    e: React.KeyboardEvent<HTMLTextAreaElement>,
    message: QueuedMessageView,
    index: number,
  ) {
    if (e.key === "Escape") {
      e.preventDefault();
      stopEdit();
    } else if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
      // Devin's shortcut: sends the message being edited immediately.
      e.preventDefault();
      saveEdit(message, index, true);
    } else if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault();
      saveEdit(message, index, false);
    }
  }

  return (
    <div className="bg-background shrink-0">
      <div className="mx-auto w-full max-w-3xl px-4 pt-2">
        <section
          aria-label={t("agentSessions.queueRegion")}
          data-testid="queued-messages"
          className="bg-muted/30 rounded-xl border"
        >
          <div className="flex items-baseline justify-between gap-3 px-3 pt-2">
            <h2 className="text-xs font-medium">
              {t("agentSessions.queueTitle", { count: messages.length })}
            </h2>
            <p className="text-muted-foreground hidden truncate text-xs sm:block">
              {t("agentSessions.queueHint")}
            </p>
          </div>
          {error ? (
            <p
              role="alert"
              className="text-destructive flex items-start gap-1.5 px-3 pt-1.5 text-xs"
            >
              <AlertCircle aria-hidden className="mt-0.5 size-3 shrink-0" />
              <span className="min-w-0 wrap-anywhere">{error}</span>
            </p>
          ) : null}
          <ul className="max-h-[40vh] space-y-1 overflow-y-auto p-1.5">
            {messages.map((message, index) => {
              const busy = busyId === message.id;
              const editing = editingId === message.id;
              return (
                <li
                  key={message.id}
                  data-testid="queued-message"
                  data-message-id={message.id}
                  className="bg-background rounded-lg border px-2 py-1.5 text-sm"
                >
                  {editing ? (
                    <div className="space-y-1.5">
                      <Textarea
                        autoFocus
                        value={draft}
                        onChange={(e) => setDraft(e.target.value)}
                        onKeyDown={(e) => handleEditKeyDown(e, message, index)}
                        aria-label={t("agentSessions.queueEditLabel")}
                        disabled={busy}
                        rows={2}
                        className="max-h-[200px] min-h-0 resize-none overflow-y-auto py-1.5"
                      />
                      <div className="flex flex-wrap items-center gap-1.5">
                        <Button
                          size="sm"
                          disabled={busy || draft.trim().length === 0}
                          onClick={() => saveEdit(message, index, false)}
                        >
                          {t("agentSessions.queueEditSave")}
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={busy || draft.trim().length === 0}
                          onClick={() => saveEdit(message, index, true)}
                        >
                          <Zap />
                          {t("agentSessions.queueEditSaveSendNow")}
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          disabled={busy}
                          onClick={stopEdit}
                        >
                          {t("agentSessions.queueEditDiscard")}
                        </Button>
                        <p className="text-muted-foreground hidden text-xs sm:block">
                          {t("agentSessions.queueEditHint")}
                        </p>
                      </div>
                    </div>
                  ) : (
                    <div className="flex flex-wrap items-start gap-x-2 gap-y-1">
                      <span
                        className="text-muted-foreground mt-0.5 w-4 shrink-0 text-xs tabular-nums"
                        title={t("agentSessions.queuePosition", {
                          position: index + 1,
                        })}
                      >
                        {index + 1}
                      </span>
                      <div className="min-w-0 flex-1 basis-40">
                        <p
                          className="line-clamp-3 break-words whitespace-pre-wrap"
                          title={message.prompt.slice(0, PREVIEW_MAX_CHARS)}
                        >
                          {message.prompt.length > PREVIEW_MAX_CHARS
                            ? `${message.prompt.slice(0, PREVIEW_MAX_CHARS)}…`
                            : message.prompt}
                        </p>
                        {message.attachmentCount > 0 ? (
                          <p className="text-muted-foreground mt-0.5 flex items-center gap-1 text-xs">
                            <Paperclip aria-hidden className="size-3" />
                            {t("agentSessions.queueAttachments", {
                              count: message.attachmentCount,
                            })}
                          </p>
                        ) : null}
                      </div>
                      <div className="ml-auto flex shrink-0 items-center gap-0.5">
                        {busy ? (
                          <Loader2
                            aria-hidden
                            className="text-muted-foreground mr-1 size-4 animate-spin"
                          />
                        ) : null}
                        <RowButton
                          label={t("agentSessions.queueSendNow")}
                          disabled={busy}
                          onClick={() =>
                            void run(message.id, () =>
                              runSendNow(message.id, index),
                            )
                          }
                        >
                          <Zap />
                        </RowButton>
                        <RowButton
                          label={t("agentSessions.queueMoveUp")}
                          disabled={busy || index === 0}
                          onClick={() =>
                            void run(message.id, async () => {
                              await move(message.id, index);
                            })
                          }
                        >
                          <ArrowUp />
                        </RowButton>
                        <RowButton
                          label={t("agentSessions.queueMoveDown")}
                          disabled={busy || index === messages.length - 1}
                          onClick={() =>
                            void run(message.id, async () => {
                              await move(message.id, index + 2);
                            })
                          }
                        >
                          <ArrowDown />
                        </RowButton>
                        <RowButton
                          label={t("agentSessions.queueEdit")}
                          disabled={busy}
                          onClick={() => startEdit(message)}
                        >
                          <Pencil />
                        </RowButton>
                        <RowButton
                          label={t("agentSessions.queueRemove")}
                          disabled={busy}
                          onClick={() =>
                            void run(message.id, () => remove(message.id))
                          }
                        >
                          <X />
                        </RowButton>
                      </div>
                      {message.blockedReason ? (
                        <p className="text-muted-foreground flex basis-full items-start gap-1.5 pl-6 text-xs">
                          <AlertCircle
                            aria-hidden
                            className="mt-0.5 size-3 shrink-0 text-amber-600 dark:text-amber-400"
                          />
                          <span className="min-w-0 wrap-anywhere">
                            {t("agentSessions.queueBlocked", {
                              reason: message.blockedReason,
                            })}
                          </span>
                        </p>
                      ) : null}
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        </section>
      </div>
    </div>
  );
}

/** One of a row's icon controls, named for assistive tech and as a tooltip. */
function RowButton({
  label,
  disabled,
  onClick,
  children,
}: {
  label: string;
  disabled: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <Button
      type="button"
      size="icon-sm"
      variant="ghost"
      className="size-7"
      aria-label={label}
      title={label}
      disabled={disabled}
      onClick={onClick}
    >
      {children}
    </Button>
  );
}

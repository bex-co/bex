import { useEffect, useMemo, useRef, useState } from "react";
import { useChat } from "@ai-sdk/react";
import type { ChatTransport } from "ai";
import {
  AlertCircle,
  Bot,
  CheckCircle2,
  ChevronDown,
  Circle,
  Loader2,
} from "lucide-react";
import {
  createAgentSessionTransport,
  type MintedTicket,
} from "@/features/agent-sessions/lib/transport";
import { useTranslations } from "@/common/hooks/use-translations";
import { MarkdownRenderer } from "@/common/components/markdown-renderer";
import { cn } from "@/common/lib/utils/utils";
import {
  ActivityGroup,
  type ActivityStep,
} from "@/features/agent-sessions/components/activity-group";
import { TypingIndicator } from "@/features/agent-sessions/components/typing-indicator";
import {
  acpDataSchema,
  acpPartData,
  classifyAcpData,
  isToolPart,
  toolPartInfo,
  type AcpPlanEntry,
  type AgentUIMessage,
} from "@/features/agent-sessions/lib/acp-parts";

// The live conversation column (ADR047 D9). `useChat` drives it over the m43
// stream transport: `resume` replays the durable transcript on mount, then
// (for a running session) live-tails; a terminal session replays and settles on
// `[DONE]`. The rendering is the polished chat shape: assistant turns get a Bot
// avatar + a markdown content column, user turns are right-aligned bubbles, and
// consecutive tool + ACP command/terminal/diff parts fold into a single
// collapsible activity group. This component is the injectable, Apollo-free unit
// under test (the transport is passed in); `session-conversation.tsx` is the
// client-only wrapper that builds the real ticket-minting transport.

/**
 * The live-steering seam the detail page lifts out of this client-only module
 * (t004): `sendMessage` submits a prompt as a live POST turn over the same
 * stream transport, and `status` lets the steering composer disable itself
 * while a turn is in flight. Exposed only while the stream is healthy — the
 * degraded/error state reports `null` so the composer's live path disables with
 * a reason instead of POSTing into a dead stream.
 */
export interface ConversationChatHandle {
  sendMessage: (text: string) => Promise<void>;
  /** The `useChat` status (`ready`/`submitted`/`streaming`/`error`). */
  status: string;
}

export interface SessionConversationImplProps {
  sessionId: string;
  /** Terminal sessions show replay-only; running sessions live-tail after replay. */
  isTerminal: boolean;
  /**
   * Mints a fresh 90s attach ticket per connection (t001's attach verb). The
   * app path passes this; the transport is built from it here so the whole
   * `ai`/transport surface stays inside this dynamically-imported module.
   */
  mintTicket?: () => Promise<MintedTicket>;
  /** A prebuilt transport — the injectable seam the unit tests drive. */
  transport?: ChatTransport<AgentUIMessage>;
  /**
   * Lifts the live-steering handle up to the detail page (t004). Called with a
   * fresh handle whenever `sendMessage`/`status` change, and with `null` when
   * the stream errors or the column unmounts, so the steering composer routes a
   * live turn through this same `useChat` instance rather than a second one.
   */
  onChatStateChange?: (handle: ConversationChatHandle | null) => void;
}

export function SessionConversationImpl({
  sessionId,
  isTerminal,
  mintTicket,
  transport: injectedTransport,
  onChatStateChange,
}: SessionConversationImplProps) {
  const { t } = useTranslations();
  // A stable transport per session: the injected one under test, else one built
  // from the ticket minter. `useMemo` keeps `useChat` from re-subscribing.
  const transport = useMemo(() => {
    if (injectedTransport) return injectedTransport;
    if (!mintTicket) {
      throw new Error(
        "SessionConversationImpl requires mintTicket or transport",
      );
    }
    return createAgentSessionTransport({ sessionId, mintTicket });
  }, [injectedTransport, mintTicket, sessionId]);

  const { messages, status, error, sendMessage } = useChat<AgentUIMessage>({
    id: sessionId,
    transport,
    // Replay the transcript on mount (GET) for both states; a running session's
    // stream then continues live, a terminal session's ends on `[DONE]`.
    resume: true,
    dataPartSchemas: { acp: acpDataSchema },
  });

  // Publish the live-steering handle to the detail page. Withheld (null) while
  // the stream is errored so the composer disables its live POST path instead of
  // steering into a dead stream; the useChat `sendMessage` appends the new turn
  // to this same transcript.
  useEffect(() => {
    if (!onChatStateChange) return;
    if (error) {
      onChatStateChange(null);
      return;
    }
    onChatStateChange({
      sendMessage: async (text: string) => {
        await sendMessage({ text });
      },
      status,
    });
  }, [onChatStateChange, sendMessage, status, error]);

  // Retract the handle on unmount so a stale send target never lingers.
  useEffect(() => {
    return () => onChatStateChange?.(null);
  }, [onChatStateChange]);

  // Auto-scroll: pin to the bottom as parts stream in, but let the user scroll
  // up to read history — a floating button returns them to the live tail. The
  // column scrolls within its own fixed-height card on the detail page.
  const messagesContainerRef = useRef<HTMLDivElement>(null);
  const isAutoScrollingRef = useRef(false);
  const [shouldAutoScroll, setShouldAutoScroll] = useState(true);

  useEffect(() => {
    if (shouldAutoScroll && messagesContainerRef.current) {
      isAutoScrollingRef.current = true;
      messagesContainerRef.current.scrollTop =
        messagesContainerRef.current.scrollHeight;
      requestAnimationFrame(() => {
        isAutoScrollingRef.current = false;
      });
    }
  }, [messages, status, shouldAutoScroll]);

  const handleScroll = () => {
    if (isAutoScrollingRef.current || !messagesContainerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } =
      messagesContainerRef.current;
    setShouldAutoScroll(Math.abs(scrollHeight - scrollTop - clientHeight) < 12);
  };

  if (error) {
    return (
      <div className="text-muted-foreground flex items-center gap-2 p-4 text-sm">
        <AlertCircle aria-hidden className="text-destructive size-4 shrink-0" />
        {t("agentSessions.conversationUnavailable")}
      </div>
    );
  }

  const empty = messages.length === 0;
  const isLoading = status === "submitted" || status === "streaming";
  const connecting = empty && isLoading;

  // Build the display blocks once per message so the parent can decide whether
  // the typing indicator is due (the last turn is a user prompt or the last
  // assistant message has no renderable content yet).
  const rendered = messages.map((message) => ({
    message,
    blocks: buildBlocks(message.parts as PartLike[]),
  }));
  const last = rendered[rendered.length - 1];
  const showTyping =
    isLoading &&
    last !== undefined &&
    (last.message.role !== "assistant" || last.blocks.length === 0);

  return (
    <div className="relative flex h-full min-h-0 flex-col">
      <div
        ref={messagesContainerRef}
        onScroll={handleScroll}
        className="min-h-0 flex-1 overflow-y-auto overscroll-contain"
        role="log"
        aria-live="polite"
      >
        <div className="space-y-6 p-4">
          {connecting && (
            <div className="text-muted-foreground flex items-center gap-2 text-sm">
              <Loader2 aria-hidden className="size-4 shrink-0 animate-spin" />
              {t("agentSessions.conversationConnecting")}
            </div>
          )}
          {empty && !connecting && (
            <p className="text-muted-foreground text-sm">
              {t("agentSessions.conversationEmpty")}
            </p>
          )}

          {rendered.map(({ message, blocks }, index) => (
            <MessageRow
              key={message.id}
              role={message.role}
              blocks={blocks}
              showCursor={isLoading && index === rendered.length - 1}
            />
          ))}

          {showTyping && (
            <div className="flex w-full items-start justify-start gap-3">
              <BotAvatar />
              <div className="border-border/50 bg-muted/60 text-muted-foreground rounded-2xl rounded-tl-md border">
                <TypingIndicator />
              </div>
            </div>
          )}
        </div>
      </div>

      {!shouldAutoScroll && !empty && (
        <button
          type="button"
          onClick={() => {
            setShouldAutoScroll(true);
            messagesContainerRef.current?.scrollTo({
              top: messagesContainerRef.current.scrollHeight,
              behavior: "smooth",
            });
          }}
          aria-label={t("agentSessions.scrollToBottom")}
          className="border-border bg-background/95 hover:bg-muted absolute bottom-4 left-1/2 z-10 flex size-8 -translate-x-1/2 items-center justify-center rounded-full border shadow-md backdrop-blur transition-colors"
        >
          <ChevronDown className="size-4" />
        </button>
      )}

      {isTerminal && status === "ready" && !empty && (
        <p className="text-muted-foreground shrink-0 border-t px-4 py-2 text-xs">
          {t("agentSessions.conversationEnded")}
        </p>
      )}
    </div>
  );
}

type PartLike = { type: string } & Record<string, unknown>;

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

// --- Display-block model ------------------------------------------------------
// A message's parts are folded into an ordered list of blocks: markdown text, a
// collapsible reasoning ("Thought") block, a plan checklist, and activity groups
// that merge every run of consecutive tool + ACP command/terminal/diff parts.

type DisplayBlock =
  | { type: "text"; key: string; text: string }
  | { type: "reasoning"; key: string; text: string }
  | { type: "plan"; key: string; entries: AcpPlanEntry[] }
  | { type: "activity"; key: string; steps: ActivityStep[] };

function buildBlocks(parts: PartLike[]): DisplayBlock[] {
  const blocks: DisplayBlock[] = [];

  const pushStep = (step: ActivityStep, index: number) => {
    const prev = blocks[blocks.length - 1];
    if (prev?.type === "activity") {
      prev.steps.push(step);
    } else {
      blocks.push({
        type: "activity",
        key: `activity-${index}`,
        steps: [step],
      });
    }
  };

  parts.forEach((part, index) => {
    if (part.type === "text") {
      blocks.push({ type: "text", key: `text-${index}`, text: str(part.text) });
      return;
    }

    if (part.type === "reasoning") {
      blocks.push({
        type: "reasoning",
        key: `reasoning-${index}`,
        text: str(part.text),
      });
      return;
    }

    const acp = acpPartData(part);
    if (acp !== undefined) {
      const group = classifyAcpData(acp);
      if (group.kind === "plan") {
        blocks.push({
          type: "plan",
          key: `plan-${index}`,
          entries: group.entries,
        });
        return;
      }
      if (group.kind === "diff") {
        pushStep(
          {
            kind: "diff",
            path: group.path,
            oldText: group.oldText,
            newText: group.newText,
          },
          index,
        );
        return;
      }
      if (group.kind === "terminal") {
        pushStep({ kind: "terminal", output: group.output }, index);
        return;
      }
      if (group.kind === "command") {
        pushStep(
          { kind: "command", title: group.title, command: group.command },
          index,
        );
        return;
      }
      pushStep({ kind: "unknown", data: group.data }, index);
      return;
    }

    if (isToolPart(part)) {
      const info = toolPartInfo(part);
      pushStep(
        {
          kind: "tool",
          name: info.name,
          state: info.state,
          input: info.input,
          output: info.output,
          errorText: info.errorText,
        },
        index,
      );
      return;
    }

    // step-start and any other structural part render nothing.
  });

  return blocks;
}

function BotAvatar() {
  return (
    <div className="shrink-0">
      <div className="border-primary/15 bg-primary/10 flex size-8 items-center justify-center rounded-full border shadow-xs">
        <Bot className="text-primary size-4" />
      </div>
    </div>
  );
}

function MessageRow({
  role,
  blocks,
  showCursor,
}: {
  role: string;
  blocks: DisplayBlock[];
  showCursor: boolean;
}) {
  const isUser = role === "user";
  // The cursor rides the final text block of the last streaming assistant turn.
  const lastTextKey = [...blocks].reverse().find((b) => b.type === "text")?.key;

  return (
    <div
      className={cn(
        "animate-in fade-in flex w-full gap-3 duration-300",
        isUser ? "justify-end pl-8" : "items-start justify-start",
      )}
    >
      {!isUser && <BotAvatar />}
      <div
        className={cn(
          "min-w-0 text-sm leading-7",
          isUser
            ? "border-border/70 bg-muted text-foreground max-w-[92%] rounded-2xl rounded-br-md border px-4 py-2.5 shadow-xs sm:max-w-md dark:bg-muted/80"
            : "min-w-0 flex-1 pt-1",
        )}
      >
        {blocks.map((block) => {
          if (block.type === "text") {
            return (
              <div key={block.key} className="min-w-0">
                <MarkdownRenderer content={block.text} />
                {!isUser && showCursor && block.key === lastTextKey && (
                  <span className="bg-current ml-1 inline-block h-4 w-2 animate-pulse" />
                )}
              </div>
            );
          }
          if (block.type === "reasoning") {
            return <ReasoningBlock key={block.key} text={block.text} />;
          }
          if (block.type === "plan") {
            return <PlanBlock key={block.key} entries={block.entries} />;
          }
          return <ActivityGroup key={block.key} steps={block.steps} />;
        })}
      </div>
    </div>
  );
}

function ReasoningBlock({ text }: { text: string }) {
  const { t } = useTranslations();
  const [isOpen, setIsOpen] = useState(false);
  return (
    <div className="border-border/70 bg-muted/20 my-3 overflow-hidden rounded-xl border">
      <button
        type="button"
        onClick={() => setIsOpen((open) => !open)}
        aria-expanded={isOpen}
        className="hover:bg-muted/40 flex w-full cursor-pointer items-center gap-2 px-3 py-2.5 text-left transition-colors"
      >
        <span className="text-foreground/90 min-w-0 flex-1 truncate text-xs font-medium">
          {t("agentSessions.groupThought")}
        </span>
        <ChevronDown
          className={cn(
            "text-muted-foreground size-3.5 shrink-0 transition-transform",
            isOpen && "rotate-180",
          )}
        />
      </button>
      {isOpen && (
        <div className="border-border/50 text-muted-foreground border-t px-3 py-2 text-sm">
          <MarkdownRenderer content={text} />
        </div>
      )}
    </div>
  );
}

function PlanBlock({ entries }: { entries: AcpPlanEntry[] }) {
  const { t } = useTranslations();
  return (
    <div className="border-border/70 bg-muted/10 my-3 rounded-xl border px-3 py-2.5">
      <p className="text-foreground/90 mb-2 text-xs font-medium">
        {t("agentSessions.groupPlan")}
      </p>
      <ul className="space-y-1.5">
        {entries.map((entry, i) => (
          <li key={i} className="flex items-start gap-2 text-sm">
            <PlanStatusIcon status={entry.status} />
            <span
              className={cn(
                "min-w-0 leading-6",
                entry.status === "completed" &&
                  "text-muted-foreground line-through",
              )}
            >
              {entry.content}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function PlanStatusIcon({ status }: { status?: string }) {
  if (status === "completed") {
    return <CheckCircle2 className="mt-0.5 size-3.5 shrink-0 text-green-600" />;
  }
  if (status === "in_progress") {
    return (
      <Loader2 className="text-primary mt-0.5 size-3.5 shrink-0 animate-spin" />
    );
  }
  return (
    <Circle className="text-muted-foreground/50 mt-0.5 size-3.5 shrink-0" />
  );
}

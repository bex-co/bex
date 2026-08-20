import {
  memo,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useChat } from "@ai-sdk/react";
import type { ChatTransport } from "ai";
import { AlertCircle, ChevronDown, Loader2, User } from "lucide-react";
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
import {
  formatStreamDuration,
  useStreamDuration,
} from "@/features/agent-sessions/lib/stream-duration";
import { TypingIndicator } from "@/features/agent-sessions/components/typing-indicator";
import {
  acpDataSchema,
  classifyAcpPart,
  isToolPart,
  sourceTimestampsMs,
  toolPartInfo,
  type AcpPlanEntry,
  type AgentUIMessage,
} from "@/features/agent-sessions/lib/acp-parts";
import { collapseDoubledParts } from "@/features/agent-sessions/lib/collapse-doubled-parts";
import {
  Task,
  TaskTrigger,
  TaskContent,
  TaskItem,
} from "@/common/components/ai-elements/task";
import {
  Reasoning,
  ReasoningTrigger,
  ReasoningContent,
} from "@/common/components/ai-elements/reasoning";

// The live conversation column (ADR047 D9). `useChat` drives it over the m43
// stream transport: `resume` replays the durable transcript on mount, then
// (for a running session) live-tails; a terminal session replays and settles on
// `[DONE]`. The rendering is the Devin chat shape: USER turns are right-aligned
// bubbles with an avatar, AGENT turns are plain full-width prose (markdown, no
// avatar, no bubble), and consecutive tool + ACP command/terminal/diff parts
// fold into a single "Worked for <Ns>" activity group. This component is the
// injectable, Apollo-free unit under test (the transport is passed in);
// `session-conversation.tsx` is the client-only wrapper that builds the real
// ticket-minting transport.

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
  /**
   * Rendered inside the scroll region after the transcript (before the terminal
   * status line) — the detail page passes the inline draft-PR card + failure
   * callout here so they read as part of the conversation flow (t005).
   */
  footer?: ReactNode;
  /**
   * Phase-derived terminus label shown (with a settled dot) once a terminal
   * session has replayed and settled. Defaults to the generic "Session ended."
   * note when unset (the injectable unit tests take the default).
   */
  terminalLabel?: string;
}

// A transcript can be as large as the gateway allows (64 MiB / thousands of
// parts). The client must not translate that straight into an unbounded React
// tree: without a budget, a crafted or simply long session parses every text and
// reasoning block as Markdown and mounts every row on every render, which can
// freeze or crash the viewer's tab (scan finding #9). Three bounds keep it safe:
//   - INITIAL_WINDOW / WINDOW_STEP cap how many message rows mount at once (older
//     rows are revealed on demand), so total DOM is bounded regardless of length;
//   - per-message block-building is memoized by message identity so a streaming
//     token re-parses only the one changed message, not the whole transcript; and
//   - MAX_BLOCK_CHARS clips a single oversized text/reasoning part before it
//     reaches the Markdown parser + DOM.
const INITIAL_WINDOW = 200;
const WINDOW_STEP = 200;

export function SessionConversationImpl({
  sessionId,
  isTerminal,
  mintTicket,
  transport: injectedTransport,
  onChatStateChange,
  footer,
  terminalLabel,
}: SessionConversationImplProps) {
  const { t } = useTranslations();
  // A stable transport per session: the injected one under test, else one built
  // from the ticket minter. `useMemo` keeps `useChat` from re-subscribing. (A
  // duplicated replay from a double resume GET is collapsed at render time by the
  // content-signature dedupe below — w3/m44.)
  const transport = useMemo(() => {
    if (injectedTransport) return injectedTransport;
    if (!mintTicket) {
      throw new Error(
        "SessionConversationImpl requires mintTicket or transport",
      );
    }
    return createAgentSessionTransport({ sessionId, mintTicket });
    // Pinned to the session, NOT `mintTicket` identity: the parent recreates the
    // minter each render (it's already memoized, so the captured one stays valid),
    // and re-keying the transport on it re-subscribed `useChat` — a second resume
    // GET that appended the replayed transcript twice (w3/m44).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [injectedTransport, sessionId]);

  const { messages, status, error, sendMessage } = useChat<AgentUIMessage>({
    id: sessionId,
    transport,
    // Replay the transcript on mount (GET) for both states; a running session's
    // stream then continues live, a terminal session's ends on `[DONE]`.
    resume: true,
    dataPartSchemas: {
      "acp-plan": acpDataSchema,
      "acp-diff": acpDataSchema,
      "acp-terminal": acpDataSchema,
      "user-prompt": acpDataSchema,
    },
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

  // Build the display blocks once per message. Collapse a duplicated replay:
  // `resume` can fire twice on mount (a second GET replays the same transcript as
  // a fresh-id message), which would render the whole conversation twice (w3/m44);
  // collapseDoubledParts dedupes by content signature so nothing real is hidden.
  //
  // Memoized by message identity (a WeakMap keyed on each message object): the AI
  // SDK keeps prior message objects referentially stable and only grows the last
  // one as tokens stream, so a streaming update rebuilds blocks for just that one
  // message instead of re-running buildBlocks over the entire transcript on every
  // token. The stable block reference is also what lets React.memo skip
  // re-rendering (and re-parsing Markdown for) every unchanged row. These hooks
  // run unconditionally (before the error early-return) per the Rules of Hooks.
  const blockCacheRef = useRef(new WeakMap<object, DisplayBlock[]>());
  const rendered = useMemo(() => {
    const cache = blockCacheRef.current;
    return messages.map((message) => {
      let blocks = cache.get(message);
      if (!blocks) {
        blocks = buildBlocks(collapseDoubledParts(message.parts as PartLike[]));
        cache.set(message, blocks);
      }
      return { message, blocks };
    });
  }, [messages]);
  // Render only the most recent window of messages; older rows are revealed on
  // demand. This bounds the mounted DOM for a very long transcript while always
  // keeping the live tail visible.
  const [windowSize, setWindowSize] = useState(INITIAL_WINDOW);

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

  const last = rendered[rendered.length - 1];
  const showTyping =
    isLoading &&
    last !== undefined &&
    (last.message.role !== "assistant" || last.blocks.length === 0);

  const hiddenCount = Math.max(0, rendered.length - windowSize);
  const visible = hiddenCount > 0 ? rendered.slice(hiddenCount) : rendered;

  return (
    <div className="relative flex h-full min-h-0 flex-col">
      <div
        ref={messagesContainerRef}
        onScroll={handleScroll}
        className="min-h-0 flex-1 overflow-y-auto overscroll-contain"
        role="log"
        aria-live="polite"
      >
        <div className="mx-auto w-full max-w-3xl space-y-2.5 px-4 py-3">
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

          {hiddenCount > 0 && (
            <button
              type="button"
              onClick={() => setWindowSize((n) => n + WINDOW_STEP)}
              className="text-muted-foreground hover:text-foreground mx-auto block text-xs underline"
            >
              {t("agentSessions.showEarlierMessages", { count: hiddenCount })}
            </button>
          )}

          {visible.map(({ message, blocks }, i) => {
            const index = hiddenCount + i;
            const isLast = index === rendered.length - 1;
            return (
              <MessageRow
                key={message.id}
                role={message.role}
                blocks={blocks}
                showCursor={isLoading && isLast}
                // A plan is "live" only in the currently-streaming last row; every
                // other (finished) row is settled, so a trailing in_progress entry
                // never spins — including historical plans once a new turn streams.
                settled={!(isLoading && isLast)}
              />
            );
          })}

          {showTyping && (
            // Agent turns carry no avatar (Devin drops it) — the typing
            // indicator sits at the prose column's left edge.
            <div className="text-muted-foreground">
              <TypingIndicator />
            </div>
          )}

          {footer}

          {isTerminal && status === "ready" && !empty && (
            <TerminalStatusLine
              label={terminalLabel ?? t("agentSessions.conversationEnded")}
            />
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
    </div>
  );
}

// The Devin-style transcript terminus: a settled dot + a phase-derived label
// ("Session went to sleep" / "…ended with an error" / "…was canceled") that ends
// a settled transcript, rendered inline at the foot of the scroll region.
function TerminalStatusLine({ label }: { label: string }) {
  return (
    <div className="text-muted-foreground flex items-center gap-2 pt-2 text-xs">
      <span
        aria-hidden
        className="bg-muted-foreground/50 size-2 shrink-0 rounded-full"
      />
      {label}
    </div>
  );
}

type PartLike = { type: string } & Record<string, unknown>;

// Largest single text/reasoning block handed to the Markdown parser. The gateway
// permits 1 MiB parts, but rich-rendering a block that big builds a huge AST +
// DOM and can freeze the tab (scan finding #9), so an oversized block is clipped
// with a neutral elision marker before it reaches the renderer. Generous enough
// that no real agent message is affected.
const MAX_BLOCK_CHARS = 100_000;

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function capText(text: string): string {
  if (text.length <= MAX_BLOCK_CHARS) return text;
  return text.slice(0, MAX_BLOCK_CHARS) + "\n\n[…]";
}

// --- Display-block model ------------------------------------------------------
// A message's parts are folded into an ordered list of blocks: markdown text, a
// collapsible reasoning ("Thought") block, a plan checklist, and activity groups
// that merge every run of consecutive tool + ACP command/terminal/diff parts.

type DisplayBlock =
  | { type: "text"; key: string; text: string }
  | { type: "reasoning"; key: string; text: string; sourceTimesMs: number[] }
  | { type: "plan"; key: string; entries: AcpPlanEntry[] }
  | {
      type: "user";
      key: string;
      text: string;
      incomplete: boolean;
      reason: string;
    }
  | {
      type: "activity";
      key: string;
      steps: ActivityStep[];
      sourceTimesMs: number[];
    };

function buildBlocks(parts: PartLike[]): DisplayBlock[] {
  const blocks: DisplayBlock[] = [];
  // ACP plans are full-state SNAPSHOTS re-sent on every update, arriving as
  // separate id-less `data-acp` parts. Render ONE plan block updated in place to
  // the latest snapshot — never a stack of stale snapshots each frozen at its
  // own (often in_progress) state (ADR051 glue #1).
  let planBlock: Extract<DisplayBlock, { type: "plan" }> | null = null;

  const pushStep = (step: ActivityStep, index: number, part: PartLike) => {
    const times = sourceTimestampsMs(part);
    const prev = blocks[blocks.length - 1];
    if (prev?.type === "activity") {
      prev.steps.push(step);
      prev.sourceTimesMs.push(...times);
    } else {
      blocks.push({
        type: "activity",
        key: `activity-${index}`,
        steps: [step],
        sourceTimesMs: [...times],
      });
    }
  };

  parts.forEach((part, index) => {
    if (part.type === "data-user-prompt") {
      const data =
        part.data && typeof part.data === "object"
          ? (part.data as Record<string, unknown>)
          : {};
      blocks.push({
        type: "user",
        key: `user-${index}`,
        text: capText(str(data.text)),
        incomplete:
          data.truncated === true ||
          (data.settled === true && data.complete !== true),
        reason: str(data.reason),
      });
      // A new durable turn starts a new plan lifecycle. Without this reset, the
      // latest plan snapshot from turn N+1 would overwrite turn N's plan block
      // because replay intentionally adapts all raw chunks into one AI-SDK
      // assistant message.
      planBlock = null;
      return;
    }

    if (part.type === "text") {
      blocks.push({
        type: "text",
        key: `text-${index}`,
        text: capText(str(part.text)),
      });
      return;
    }

    if (part.type === "reasoning") {
      blocks.push({
        type: "reasoning",
        key: `reasoning-${index}`,
        text: capText(str(part.text)),
        sourceTimesMs: sourceTimestampsMs(part),
      });
      return;
    }

    const group = classifyAcpPart(part);
    if (group) {
      if (group.kind === "plan") {
        if (planBlock) {
          planBlock.entries = group.entries; // supersede with the latest snapshot
        } else {
          planBlock = {
            type: "plan",
            key: `plan-${index}`,
            entries: group.entries,
          };
          blocks.push(planBlock);
        }
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
          part,
        );
        return;
      }
      pushStep(
        { kind: "terminal", output: group.output },
        index,
        part,
      );
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
        part,
      );
      return;
    }

    // step-start and any other structural part render nothing.
  });

  return blocks;
}

// The user's own avatar — Devin keeps an avatar on the right-aligned USER
// bubble, but drops it entirely for the agent's plain-prose turns.
function UserAvatar() {
  return (
    <div className="shrink-0">
      <div className="border-border/60 bg-muted flex size-8 items-center justify-center rounded-full border shadow-xs">
        <User className="text-muted-foreground size-4" />
      </div>
    </div>
  );
}

// Memoized so a streaming update — which changes only the last row's props —
// does not re-render (and re-parse the Markdown of) every earlier row. This
// relies on buildBlocks being memoized by message identity above, so an
// unchanged row's `blocks` reference is stable across renders (scan finding #9).
const MessageRow = memo(function MessageRow({
  role,
  blocks,
  showCursor,
  settled,
}: {
  role: string;
  blocks: DisplayBlock[];
  showCursor: boolean;
  settled: boolean;
}) {
  const isUser = role === "user";
  // The cursor rides the final text block of the last streaming assistant turn.
  const lastTextKey = [...blocks].reverse().find((b) => b.type === "text")?.key;

  return (
    <div
      className={cn(
        "animate-in fade-in flex w-full gap-2 duration-300",
        // USER: right-aligned bubble + avatar. AGENT: plain full-width prose,
        // no avatar, no bubble — the whole pane is the conversation.
        isUser ? "justify-end pl-8" : "justify-start",
      )}
    >
      <div
        className={cn(
          "min-w-0 text-sm leading-6",
          isUser
            ? "border-border/70 bg-muted text-foreground max-w-[92%] rounded-xl rounded-br-md border px-3 py-1.5 shadow-xs sm:max-w-md dark:bg-muted/80"
            : "min-w-0 flex-1",
        )}
      >
        {blocks.map((block) => {
          if (block.type === "user") {
            return (
              <DurableUserTurn
                key={block.key}
                text={block.text}
                incomplete={block.incomplete}
                reason={block.reason}
              />
            );
          }
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
            return (
              <ReasoningBlock
                key={block.key}
                text={block.text}
                streaming={showCursor}
                sourceTimesMs={block.sourceTimesMs}
              />
            );
          }
          if (block.type === "plan") {
            return (
              <PlanBlock
                key={block.key}
                entries={block.entries}
                settled={settled}
              />
            );
          }
          return (
            <ActivityGroup
              key={block.key}
              steps={block.steps}
              sourceTimesMs={block.sourceTimesMs}
            />
          );
        })}
      </div>
      {isUser && <UserAvatar />}
    </div>
  );
});

function DurableUserTurn({
  text,
  incomplete,
  reason,
}: {
  text: string;
  incomplete: boolean;
  reason: string;
}) {
  const { t } = useTranslations();
  return (
    <div className="my-2 flex w-full justify-end gap-2 pl-8">
      <div className="max-w-[92%] sm:max-w-md">
        <div className="border-border/70 bg-muted text-foreground rounded-xl rounded-br-md border px-3 py-1.5 shadow-xs dark:bg-muted/80">
          <MarkdownRenderer content={text} />
        </div>
        {incomplete ? (
          <p className="text-muted-foreground mt-1 text-right text-xs">
            {t("agentSessions.conversationIncomplete")}
            {reason ? `: ${reason}` : ""}
          </p>
        ) : null}
      </div>
      <UserAvatar />
    </div>
  );
}

// The agent's chain-of-thought, rendered with the vendored AI Elements
// `Reasoning` disclosure. Collapsed by default so a long transcript stays
// scannable; the "Thought for <Ns>" label prefers persisted source timestamps.
function ReasoningBlock({
  text,
  streaming,
  sourceTimesMs,
}: {
  text: string;
  streaming: boolean;
  sourceTimesMs: number[];
}) {
  const { t } = useTranslations();
  const duration = useStreamDuration(text.length, !streaming, sourceTimesMs);
  const summary =
    duration.ms >= 1000
      ? t("agentSessions.groupThoughtFor", {
          duration: formatStreamDuration(duration),
        })
      : t("agentSessions.groupThought");
  return (
    <Reasoning className="my-1.5">
      <ReasoningTrigger>{summary}</ReasoningTrigger>
      <ReasoningContent>
        <MarkdownRenderer content={text} />
      </ReasoningContent>
    </Reasoning>
  );
}

// The agent's plan, rendered with the vendored AI Elements `Task` checklist.
// Once the session has settled, a trailing `in_progress` entry drops to a
// neutral (pending) icon instead of a live loader — ACP agents routinely end a
// turn with the last step still in_progress, and the platform (not the agent)
// opens the PR, so the stream never flips it to completed (ADR051 glue #1).
function PlanBlock({
  entries,
  settled,
}: {
  entries: AcpPlanEntry[];
  settled: boolean;
}) {
  const { t } = useTranslations();
  return (
    <Task defaultOpen className="my-1.5">
      <TaskTrigger title={t("agentSessions.groupPlan")} />
      <TaskContent>
        {entries.map((entry, i) => (
          <TaskItem key={i} status={planItemStatus(entry.status, settled)}>
            {entry.content}
          </TaskItem>
        ))}
      </TaskContent>
    </Task>
  );
}

function planItemStatus(status: string | undefined, settled: boolean): string {
  if (settled && status === "in_progress") return "pending";
  return status ?? "pending";
}

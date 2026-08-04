import { Fragment, useEffect, useMemo } from "react";
import { useChat } from "@ai-sdk/react";
import type { ChatTransport } from "ai";
import { AlertCircle, Loader2 } from "lucide-react";
import {
  createAgentSessionTransport,
  type MintedTicket,
} from "@/features/agent-sessions/lib/transport";
import { useTranslations } from "@/common/hooks/use-translations";
import { CodeBlock } from "@/common/components/code-block";
import {
  Conversation,
  ConversationContent,
  Message,
  MessageContent,
  Response,
  Reasoning,
  ReasoningTrigger,
  ReasoningContent,
  Task,
  TaskTrigger,
  TaskContent,
  TaskItem,
  Tool,
  ToolHeader,
  ToolContent,
  ToolInput,
  ToolOutput,
  Terminal,
  TerminalTrigger,
  TerminalContent,
} from "@/common/components/ai-elements";
import {
  acpDataSchema,
  acpPartData,
  classifyAcpData,
  isToolPart,
  toolPartInfo,
  type AgentUIMessage,
} from "@/features/agent-sessions/lib/acp-parts";

// The live conversation column (ADR047 D9). `useChat` drives it over the m43
// stream transport: `resume` replays the durable transcript on mount, then
// (for a running session) live-tails; a terminal session replays and settles on
// `[DONE]`. Each `data-acp` part and tool/reasoning UI part renders as a
// collapsible group — the Devin "Worked / Thought" shape. This component is the
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

  if (error) {
    return (
      <div className="text-muted-foreground flex items-center gap-2 p-4 text-sm">
        <AlertCircle aria-hidden className="text-destructive size-4 shrink-0" />
        {t("agentSessions.conversationUnavailable")}
      </div>
    );
  }

  const empty = messages.length === 0;
  const connecting =
    empty && (status === "submitted" || status === "streaming");

  return (
    <Conversation className="min-h-0">
      <ConversationContent>
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
        {messages.map((message) => (
          <Message key={message.id} from={message.role}>
            <MessageContent from={message.role}>
              {message.parts.map((part, index) => (
                <Fragment key={`${message.id}-${index}`}>
                  <PartView part={part as PartLike} />
                </Fragment>
              ))}
            </MessageContent>
          </Message>
        ))}
      </ConversationContent>
      {isTerminal && status === "ready" && !empty && (
        <p className="text-muted-foreground border-t px-4 py-2 text-xs">
          {t("agentSessions.conversationEnded")}
        </p>
      )}
    </Conversation>
  );
}

type PartLike = { type: string } & Record<string, unknown>;

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

// Renders one message part into its transcript group. A component (not a bare
// function) so each collapsible owns its own open state across re-renders.
function PartView({ part }: { part: PartLike }) {
  const { t } = useTranslations();

  if (part.type === "text") {
    return <Response>{str(part.text)}</Response>;
  }

  if (part.type === "reasoning") {
    return (
      <Reasoning>
        <ReasoningTrigger>{t("agentSessions.groupThought")}</ReasoningTrigger>
        <ReasoningContent>{str(part.text)}</ReasoningContent>
      </Reasoning>
    );
  }

  if (isToolPart(part)) {
    const info = toolPartInfo(part);
    return (
      <Tool>
        <ToolHeader
          name={info.name}
          state={info.state}
          stateLabel={toolStateLabel(info.state, t)}
        />
        <ToolContent>
          <ToolInput label={t("agentSessions.toolInput")} input={info.input} />
          <ToolOutput
            label={t("agentSessions.toolOutput")}
            errorLabel={t("agentSessions.toolError")}
            output={info.output}
            errorText={info.errorText}
          />
        </ToolContent>
      </Tool>
    );
  }

  const acp = acpPartData(part);
  if (acp !== undefined) {
    const group = classifyAcpData(acp);
    switch (group.kind) {
      case "plan":
        return (
          <Task>
            <TaskTrigger title={t("agentSessions.groupPlan")} />
            <TaskContent>
              {group.entries.map((entry, i) => (
                <TaskItem key={i} status={entry.status}>
                  {entry.content}
                </TaskItem>
              ))}
            </TaskContent>
          </Task>
        );
      case "terminal":
        return (
          <Terminal>
            <TerminalTrigger>
              {t("agentSessions.groupTerminal")}
            </TerminalTrigger>
            <TerminalContent>
              {group.output ?? t("agentSessions.terminalNoOutput")}
            </TerminalContent>
          </Terminal>
        );
      case "command":
        return (
          <Terminal>
            <TerminalTrigger>
              {group.title ?? t("agentSessions.groupCommand")}
            </TerminalTrigger>
            <TerminalContent>
              {group.command ?? group.title ?? ""}
            </TerminalContent>
          </Terminal>
        );
      case "diff":
        return (
          <div className="space-y-1">
            <p className="text-muted-foreground text-xs font-medium">
              {group.path ?? t("agentSessions.groupDiff")}
            </p>
            <CodeBlock
              code={unifiedDiff(group.oldText, group.newText)}
              language="diff"
            />
          </div>
        );
      default:
        return <CodeBlock code={safeJson(group.data)} language="json" />;
    }
  }

  // step-start and any other structural part render nothing.
  return null;
}

function toolStateLabel(
  state: string,
  t: ReturnType<typeof useTranslations>["t"],
): string {
  switch (state) {
    case "input-streaming":
    case "input-available":
      return t("agentSessions.toolStateRunning");
    case "output-available":
      return t("agentSessions.toolStateDone");
    case "output-error":
      return t("agentSessions.toolStateError");
    default:
      return state;
  }
}

// unifiedDiff renders a minimal +/- diff from the ACP diff part's old/new text
// so the CodeBlock reads like a patch without a diffing dependency.
function unifiedDiff(oldText?: string, newText?: string): string {
  const removed = (oldText ?? "")
    .split("\n")
    .filter((l) => l.length > 0)
    .map((l) => `- ${l}`);
  const added = (newText ?? "")
    .split("\n")
    .filter((l) => l.length > 0)
    .map((l) => `+ ${l}`);
  return [...removed, ...added].join("\n") || "(no changes)";
}

function safeJson(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

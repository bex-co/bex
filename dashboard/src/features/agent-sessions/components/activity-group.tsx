import { useState } from "react";
import {
  ChevronDown,
  Clock,
  FileDiff,
  Loader2,
  Terminal as TerminalIcon,
} from "lucide-react";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  formatApproxDuration,
  useStreamDuration,
} from "@/features/agent-sessions/lib/stream-duration";
import { unwrapAcpTool } from "@/features/agent-sessions/lib/acp-parts";
import {
  Tool,
  ToolHeader,
  ToolContent,
  ToolInput,
  ToolOutput,
} from "@/common/components/ai-elements/tool";
import {
  Terminal,
  TerminalTrigger,
  TerminalContent,
} from "@/common/components/ai-elements/terminal";

// One folded activity block: the consecutive tool parts and ACP
// command/terminal/diff parts of a single assistant turn, merged into a single
// collapsible group (Devin's "Worked for <Ns>" shape). The collapsed summary
// states "Working…" while any tool step is pending, else "Worked for ~Ns" with a
// derived duration (or a bare "Worked" when no duration could be derived, see
// `useStreamDuration`). Expanding reveals the steps as a VERTICAL TIMELINE — a
// connector line with a node per step. The derived-duration mechanism lives in
// `lib/stream-duration.ts` (t004 — no per-part timestamps in the m43 transcript).

/** One renderable step inside an activity group. */
export type ActivityStep =
  | {
      kind: "tool";
      name: string;
      state: string;
      input?: unknown;
      output?: unknown;
      errorText?: string;
    }
  | { kind: "command"; title?: string; command?: string }
  | { kind: "terminal"; output?: string }
  | { kind: "diff"; path?: string; oldText?: string; newText?: string }
  | { kind: "unknown"; data: unknown };

/** A tool step is pending until it reaches a terminal (output/error) state. */
function isToolStepPending(step: ActivityStep): boolean {
  return (
    step.kind === "tool" &&
    step.state !== "output-available" &&
    step.state !== "output-error"
  );
}

export function ActivityGroup({ steps }: { steps: ActivityStep[] }) {
  const { t } = useTranslations();
  const [isOpen, setIsOpen] = useState(false);

  const hasPending = steps.some(isToolStepPending);
  // Derive the "Worked for <Ns>" duration from stream-arrival timing; freeze it
  // once every step has settled (no tool call still pending).
  const durationMs = useStreamDuration(steps.length, !hasPending);

  const summaryLabel = hasPending
    ? t("agentSessions.activityWorking")
    : durationMs >= 1000
      ? t("agentSessions.groupWorkedFor", {
          duration: formatApproxDuration(durationMs),
        })
      : t("agentSessions.groupWorked");

  return (
    <div className="bg-muted/20 border-border/70 my-2 overflow-hidden rounded-lg border">
      <button
        type="button"
        onClick={() => setIsOpen((open) => !open)}
        aria-expanded={isOpen}
        className="hover:bg-muted/40 flex w-full cursor-pointer items-center gap-2 px-2.5 py-1.5 text-left transition-colors"
      >
        {hasPending ? (
          <Loader2 className="text-muted-foreground size-3.5 shrink-0 animate-spin" />
        ) : (
          <Clock className="text-muted-foreground size-3.5 shrink-0" />
        )}
        <span className="text-foreground/90 min-w-0 flex-1 truncate text-xs font-medium">
          {summaryLabel}
        </span>
        <ChevronDown
          className={cn(
            "text-muted-foreground size-3.5 shrink-0 transition-transform",
            isOpen && "rotate-180",
          )}
        />
      </button>

      {isOpen && (
        <div className="border-border/50 border-t px-2.5 py-2">
          {/* Vertical timeline: a connector line down the left with a node per
              step, the individual steps rendered in order. */}
          <ol className="border-border/60 relative ml-1 space-y-1.5 border-l pl-4">
            {steps.map((step, i) => (
              <li key={i} className="relative">
                <span
                  aria-hidden
                  className="bg-border ring-background absolute top-2 -left-[1.3rem] size-2 rounded-full ring-2"
                />
                <ActivityStepView step={step} />
              </li>
            ))}
          </ol>
        </div>
      )}
    </div>
  );
}

function ActivityStepView({ step }: { step: ActivityStep }) {
  const { t } = useTranslations();

  if (step.kind === "command") {
    const line = step.command ?? step.title ?? "";
    return (
      <StepShell icon={<TerminalIcon className="text-primary/60 size-3" />}>
        <span className="font-semibold">
          {step.title ?? t("agentSessions.groupCommand")}
        </span>
        {line && <code className="truncate">{line}</code>}
      </StepShell>
    );
  }

  if (step.kind === "terminal") {
    // Vendored AI Elements terminal block (dark shell pane), open inline within
    // the already-collapsed activity group.
    return (
      <Terminal defaultOpen>
        <TerminalTrigger>{t("agentSessions.groupTerminal")}</TerminalTrigger>
        <TerminalContent>
          {step.output ?? t("agentSessions.terminalNoOutput")}
        </TerminalContent>
      </Terminal>
    );
  }

  if (step.kind === "diff") {
    return (
      <div className="space-y-1">
        <StepLabel icon={<FileDiff className="text-primary/60 size-3" />}>
          {step.path ?? t("agentSessions.groupDiff")}
        </StepLabel>
        <StepCode code={unifiedDiff(step.oldText, step.newText)} />
      </div>
    );
  }

  if (step.kind === "tool") {
    // unwrapAcpTool recovers the real tool name/command and drops trivial acks
    // (see its docstring); render via the vendored AI Elements Tool, open inline.
    const tool = unwrapAcpTool({
      name: step.name,
      state: step.state,
      input: step.input,
      output: step.output,
      errorText: step.errorText,
    });
    return (
      <Tool defaultOpen>
        <ToolHeader
          name={tool.name}
          state={tool.state}
          stateLabel={toolStateLabel(tool.state, t)}
        />
        <ToolContent>
          {tool.command !== undefined ? (
            <ToolInput
              label={t("agentSessions.toolCommand")}
              input={tool.command}
            />
          ) : (
            <ToolInput label={t("agentSessions.toolInput")} input={tool.args} />
          )}
          <ToolOutput
            label={t("agentSessions.toolOutput")}
            errorLabel={t("agentSessions.toolError")}
            output={tool.output}
            errorText={tool.errorText}
          />
        </ToolContent>
      </Tool>
    );
  }

  return <StepCode code={safeJson(step.data)} />;
}

// A compact single-line step row (command / tool header).
function StepShell({
  icon,
  children,
}: {
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="border-border/60 bg-background text-muted-foreground rounded-md border px-2.5 py-1.5 font-mono text-xs">
      <div className="flex items-center gap-1.5">
        {icon}
        {children}
      </div>
    </div>
  );
}

// A compact code body for a timeline step. The shared CodeBlock's chrome
// (language header bar, p-4, my-4) triples a step's height; a step is context,
// not a document, so it gets a bare scroll-capped pre instead (w3 compact pass).
function StepCode({ code }: { code: string }) {
  return (
    <pre className="border-border/60 bg-background text-muted-foreground max-h-48 overflow-auto rounded-md border px-2.5 py-1.5 font-mono text-xs whitespace-pre-wrap">
      {code}
    </pre>
  );
}

// A small caption above a block-level step body (terminal output / diff).
function StepLabel({
  icon,
  children,
}: {
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="text-muted-foreground flex items-center gap-1.5 text-xs font-medium">
      {icon}
      <span className="truncate">{children}</span>
    </div>
  );
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

// Renders a minimal +/- unified diff from the ACP diff part's old/new text so
// CodeBlock reads like a patch without a diffing dependency.
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

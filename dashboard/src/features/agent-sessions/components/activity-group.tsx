import { useState } from "react";
import {
  ChevronDown,
  Clock,
  FileDiff,
  Loader2,
  Terminal as TerminalIcon,
  Wrench,
} from "lucide-react";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  formatStreamDuration,
  useStreamDuration,
} from "@/features/agent-sessions/lib/stream-duration";
import { unwrapAcpTool } from "@/features/agent-sessions/lib/acp-parts";

// One folded activity block: the consecutive tool parts and ACP
// command/terminal/diff parts of a single assistant turn, merged into a single
// collapsible group (Devin's "Worked for <Ns>" shape). The collapsed summary
// states "Working…" while any tool step is pending, else "Worked for 12s" when
// persisted source timestamps are present (or "Worked for ~Ns" from arrival
// timing). A one-frame replay of an untimestamped history still falls back to
// a bare "Worked". Expanding reveals the steps as a VERTICAL TIMELINE.

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

export function ActivityGroup({
  steps,
  sourceTimesMs = [],
}: {
  steps: ActivityStep[];
  sourceTimesMs?: readonly number[];
}) {
  const { t } = useTranslations();
  const [isOpen, setIsOpen] = useState(false);

  const hasPending = steps.some(isToolStepPending);
  const duration = useStreamDuration(steps.length, !hasPending, sourceTimesMs);

  const summaryLabel = hasPending
    ? t("agentSessions.activityWorking")
    : duration.ms >= 1000
      ? t("agentSessions.groupWorkedFor", {
          duration: formatStreamDuration(duration),
        })
      : t("agentSessions.groupWorked");

  return (
    <div className="bg-muted/20 border-border/70 my-1.5 overflow-hidden rounded-lg border">
      <button
        type="button"
        onClick={() => setIsOpen((open) => !open)}
        aria-expanded={isOpen}
        className="hover:bg-muted/40 flex w-full cursor-pointer items-center gap-2 px-2.5 py-1 text-left transition-colors"
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
        <div className="border-border/50 border-t px-2.5 py-1.5">
          {/* Vertical timeline: a connector line down the left with a node per
              step, the individual steps rendered in order. */}
          <ol className="border-border/60 relative ml-1 space-y-1 border-l pl-4">
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
    return (
      <div className="space-y-1">
        <StepLabel icon={<TerminalIcon className="text-primary/60 size-3" />}>
          {t("agentSessions.groupTerminal")}
        </StepLabel>
        <pre className="max-h-40 overflow-auto rounded-md bg-[#0a0a0a] px-2.5 py-1.5 font-mono text-xs leading-relaxed whitespace-pre-wrap text-[#e5e5e5]">
          {step.output ?? t("agentSessions.terminalNoOutput")}
        </pre>
      </div>
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
    // (see its docstring). Rendered as a compact single-line row: a lifted shell
    // command sits inline (no CodeBlock chrome); only non-trivial args/output get
    // a bare capped pre below.
    const tool = unwrapAcpTool({
      name: step.name,
      state: step.state,
      input: step.input,
      output: step.output,
      errorText: step.errorText,
    });
    const running =
      tool.state !== "output-available" && tool.state !== "output-error";
    return (
      <div className="space-y-1">
        <StepShell
          icon={
            running ? (
              <Loader2 className="size-3 shrink-0 animate-spin" />
            ) : (
              <Wrench className="text-primary/60 size-3 shrink-0" />
            )
          }
        >
          <span className="font-semibold">{tool.name}</span>
          {tool.command && (
            <code className="min-w-0 truncate">{tool.command}</code>
          )}
          <span className="text-muted-foreground/60 ml-auto shrink-0">
            {toolStateLabel(tool.state, t)}
          </span>
        </StepShell>
        {tool.command === undefined && tool.args !== undefined && (
          <StepCode code={safeJson(tool.args)} />
        )}
        {tool.output !== undefined && <StepCode code={safeJson(tool.output)} />}
        {tool.errorText && (
          <p className="text-destructive text-xs">{tool.errorText}</p>
        )}
      </div>
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
    <div className="border-border/60 bg-background text-muted-foreground rounded-md border px-2 py-1 font-mono text-xs">
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
    <pre className="border-border/60 bg-background text-muted-foreground max-h-40 overflow-auto rounded-md border px-2 py-1 font-mono text-xs whitespace-pre-wrap">
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

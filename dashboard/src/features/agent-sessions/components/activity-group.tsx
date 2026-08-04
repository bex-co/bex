import { useState } from "react";
import {
  Bot,
  ChevronDown,
  FileDiff,
  Loader2,
  Terminal as TerminalIcon,
  Wrench,
} from "lucide-react";
import { cn } from "@/common/lib/utils/utils";
import { CodeBlock } from "@/common/components/code-block";
import { useTranslations } from "@/common/hooks/use-translations";

// One folded activity block: the consecutive tool parts and ACP
// command/terminal/diff parts of a single assistant turn, merged into a single
// collapsible card (the reference's "grouped activity" shape). The collapsed
// summary row states what happened ("Working…" while any tool is pending, else
// "Edited N files" / "Ran N commands" / "N steps"); expanding reveals each step.

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
  const diffCount = steps.filter((s) => s.kind === "diff").length;
  const commandCount = steps.filter(
    (s) => s.kind === "command" || s.kind === "terminal",
  ).length;

  const summaryLabel = hasPending
    ? t("agentSessions.activityWorking")
    : diffCount > 0
      ? t("agentSessions.activityEdited", { count: diffCount })
      : commandCount > 0
        ? t("agentSessions.activityRan", { count: commandCount })
        : t("agentSessions.activitySteps", { count: steps.length });

  return (
    <div className="bg-muted/20 border-border/70 my-3 overflow-hidden rounded-xl border">
      <button
        type="button"
        onClick={() => setIsOpen((open) => !open)}
        aria-expanded={isOpen}
        className="hover:bg-muted/40 flex w-full cursor-pointer items-center gap-2 px-3 py-2.5 text-left transition-colors"
      >
        {hasPending ? (
          <Loader2 className="text-muted-foreground size-3.5 shrink-0 animate-spin" />
        ) : (
          <Bot className="text-muted-foreground size-3.5 shrink-0" />
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
        <div className="border-border/50 space-y-2 border-t px-3 py-2">
          {steps.map((step, i) => (
            <ActivityStepView key={i} step={step} />
          ))}
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
        <CodeBlock
          code={step.output ?? t("agentSessions.terminalNoOutput")}
          language="bash"
        />
      </div>
    );
  }

  if (step.kind === "diff") {
    return (
      <div className="space-y-1">
        <StepLabel icon={<FileDiff className="text-primary/60 size-3" />}>
          {step.path ?? t("agentSessions.groupDiff")}
        </StepLabel>
        <CodeBlock
          code={unifiedDiff(step.oldText, step.newText)}
          language="diff"
        />
      </div>
    );
  }

  if (step.kind === "tool") {
    const running =
      step.state !== "output-available" && step.state !== "output-error";
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
          <span className="font-semibold">{step.name}</span>
          <span className="text-muted-foreground/70">
            {toolStateLabel(step.state, t)}
          </span>
        </StepShell>
        {step.input !== undefined && step.input !== null && (
          <CodeBlock code={safeJson(step.input)} language="json" />
        )}
        {step.output !== undefined && step.output !== null && (
          <CodeBlock code={safeJson(step.output)} language="json" />
        )}
        {step.errorText && (
          <p className="text-destructive text-xs">{step.errorText}</p>
        )}
      </div>
    );
  }

  return <CodeBlock code={safeJson(step.data)} language="json" />;
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
    <div className="border-border/60 bg-background text-muted-foreground rounded-lg border px-3 py-2 font-mono text-xs">
      <div className="flex items-center gap-1.5">
        {icon}
        {children}
      </div>
    </div>
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

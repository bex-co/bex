import { useState } from "react";
import {
  ChevronDown,
  Clock,
  FileDiff,
  Loader2,
  Terminal as TerminalIcon,
  Wrench,
  XCircle,
} from "lucide-react";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  formatStreamDuration,
  useStreamDuration,
} from "@/features/agent-sessions/lib/stream-duration";
import {
  clippedChars,
  toolCommand,
  type ToolKind,
} from "@/features/agent-sessions/lib/acp-parts";
import {
  activityCounts,
  isToolStepFailed,
  isToolStepPending,
  stepLabel,
  type ActivityStep,
} from "@/features/agent-sessions/lib/activity-steps";

// One folded activity block: the consecutive tool parts and ACP
// command/terminal/diff parts of a single assistant turn, merged into ONE
// signal-first summary line (w10/m16 t005): "Ran 12 commands · Read 8 files ·
// Edited 3 files", with the elapsed time beside it ("40s" from persisted source
// timestamps, "~40s" from arrival timing, nothing on an untimestamped one-frame
// replay). Collapsed by default; failed calls stay visible under the line
// without expanding. Expanding reveals the steps as a VERTICAL TIMELINE.

const KIND_ORDER: ToolKind[] = [
  "command",
  "read",
  "edit",
  "search",
  "subagent",
  "tool",
];

const KIND_LABEL_KEYS: Record<ToolKind, string> = {
  command: "agentSessions.activityRan",
  read: "agentSessions.activityRead",
  edit: "agentSessions.activityEdited",
  search: "agentSessions.activitySearched",
  subagent: "agentSessions.activitySubagents",
  tool: "agentSessions.activityTools",
};

export function ActivityGroup({
  steps,
  sourceTimesMs = [],
  interrupted = false,
}: {
  steps: ActivityStep[];
  sourceTimesMs?: readonly number[];
  /**
   * The conversation has settled for good (a terminal session's finished
   * replay): a call still pending — a partial item the harvest never completed
   * — will never finish, so it renders Interrupted instead of spinning.
   */
  interrupted?: boolean;
}) {
  const { t } = useTranslations();
  const [isOpen, setIsOpen] = useState(false);

  const hasPending = !interrupted && steps.some(isToolStepPending);
  const duration = useStreamDuration(steps.length, !hasPending, sourceTimesMs);
  const failed = steps.filter(isToolStepFailed);
  const counts = activityCounts(steps);
  const segments = KIND_ORDER.flatMap((kind) => {
    const count = counts.get(kind);
    return count ? [t(KIND_LABEL_KEYS[kind] as never, { count })] : [];
  });
  const summaryLabel = segments.join(" · ");

  return (
    <div
      data-block="activity"
      data-steps={steps.length}
      className="bg-muted/20 border-border/70 my-1.5 overflow-hidden rounded-lg border"
    >
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
          {failed.length > 0 && (
            <span className="text-destructive">
              {" · "}
              {t("agentSessions.activityFailed", { count: failed.length })}
            </span>
          )}
        </span>
        {!hasPending && duration.ms >= 1000 && (
          <span className="text-muted-foreground shrink-0 text-xs tabular-nums">
            {formatStreamDuration(duration)}
          </span>
        )}
        <ChevronDown
          className={cn(
            "text-muted-foreground size-3.5 shrink-0 transition-transform",
            isOpen && "rotate-180",
          )}
        />
      </button>

      {/* A failure is signal: visible without expanding the group. */}
      {!isOpen && failed.length > 0 && (
        <ul className="border-border/50 space-y-0.5 border-t px-2.5 py-1">
          {failed.map((step, i) => (
            <li
              key={step.toolCallId ?? i}
              className="text-destructive flex min-w-0 items-center gap-1.5 text-xs"
            >
              <XCircle aria-hidden className="size-3 shrink-0" />
              <span className="min-w-0 truncate">
                <span className="font-medium">{stepLabel(step)}</span>
                {step.errorText ? `: ${firstLine(step.errorText)}` : ""}
              </span>
            </li>
          ))}
        </ul>
      )}

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
                <ActivityStepView step={step} interrupted={interrupted} />
              </li>
            ))}
          </ol>
        </div>
      )}
    </div>
  );
}

// The first non-empty, non-fence line of an error (outputs often open with a
// ``` fence).
function firstLine(text: string): string {
  return (
    text
      .split("\n")
      .map((line) => line.trim())
      .find((line) => line && !line.startsWith("```")) ?? ""
  );
}

function ActivityStepView({
  step,
  interrupted,
}: {
  step: ActivityStep;
  interrupted: boolean;
}) {
  const { t } = useTranslations();

  if (step.kind === "terminal") {
    return (
      <div className="space-y-1">
        <StepLabel icon={<TerminalIcon className="text-primary/60 size-3" />}>
          {t("agentSessions.groupTerminal")}
        </StepLabel>
        <pre className="max-h-40 overflow-auto rounded-md bg-[#0a0a0a] px-2.5 py-1.5 font-mono text-xs leading-relaxed whitespace-pre-wrap text-[#e5e5e5]">
          {step.output ?? t("agentSessions.terminalNoOutput")}
        </pre>
        <ClippedBadge chars={clippedChars(step.output)} />
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
    const command = toolCommand(step.input);
    const pending = isToolStepPending(step);
    const running = pending && !interrupted;
    // Bare success acknowledgements add no detail to the visible Done state.
    const output = step.output;
    const ack =
      output == null ||
      output === true ||
      (typeof output === "object" &&
        !Array.isArray(output) &&
        Object.entries(output).every(
          ([key, value]) =>
            ["ok", "success", "status"].includes(key) &&
            (value === true || value === "ok" || value === "success"),
        ));
    // A structured output is clipped string by string, so its JSON text
    // carries the markers too.
    const outputText = ack
      ? undefined
      : typeof output === "string"
        ? output
        : safeJson(output);
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
          {step.name !== command && (
            <span className="min-w-0 truncate font-semibold">{step.name}</span>
          )}
          {command && <code className="min-w-0 truncate">{command}</code>}
          <span className="text-muted-foreground/60 ml-auto shrink-0">
            {pending && interrupted
              ? t("agentSessions.toolStateInterrupted")
              : toolStateLabel(step.state, t)}
          </span>
        </StepShell>
        {command === undefined && step.input !== undefined && (
          <StepCode code={safeJson(step.input)} />
        )}
        {outputText !== undefined && <StepCode code={outputText} />}
        {step.errorText && (
          <p className="text-destructive text-xs whitespace-pre-wrap">
            {step.errorText}
          </p>
        )}
        <ClippedBadge
          chars={clippedChars(outputText) + clippedChars(step.errorText)}
        />
      </div>
    );
  }
}

// The server stores a long tool output head+tail-capped with a
// `[bex: N chars clipped]` marker (w10/m16 t003); say so beside the output.
function ClippedBadge({ chars }: { chars: number }) {
  const { t, i18n } = useTranslations();
  if (chars <= 0) return null;
  return (
    <span className="border-border/60 text-muted-foreground inline-flex rounded border px-1.5 py-px text-[11px]">
      {t("agentSessions.outputClipped", {
        count: chars,
        formatted: chars.toLocaleString(i18n.language),
      })}
    </span>
  );
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
    case "output-denied":
      return t("agentSessions.toolStateDenied");
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

import {
  toolCommand,
  toolKind,
  type ToolKind,
  type ToolPartInfo,
} from "@/features/agent-sessions/lib/acp-parts";

// The activity-group step model and its signal-first summary (w10/m16 t005):
// pure helpers shared by the ActivityGroup component and the conversation's
// live status line.

/**
 * Where a clipped step's complete output is stored out of line (w10/m18): the
 * durable turn it belongs to and the field under that turn's tool call
 * (`output`/`errorText`, or `terminal:<terminalId>`). `fullOutput` is set only
 * when the replayed part says the server kept a full copy; an output clipped
 * before full copies were kept has none.
 */
export interface FullOutputRef {
  turn?: number;
  fullOutput?: boolean;
}

/** One renderable step inside an activity group. */
export type ActivityStep =
  | ({ kind: "tool"; toolCallId?: string } & ToolPartInfo & FullOutputRef)
  | ({
      kind: "terminal";
      toolCallId?: string;
      terminalId?: string;
      output?: string;
    } & FullOutputRef)
  | {
      kind: "diff";
      toolCallId?: string;
      path?: string;
      oldText?: string;
      newText?: string;
    };

// The AI SDK v6 tool states a call never leaves: an output, an error, or a
// denied execution (`tool-output-denied`).
const SETTLED_TOOL_STATES = new Set([
  "output-available",
  "output-error",
  "output-denied",
]);

/** A tool step is pending until it reaches a terminal (output/error/denied) state. */
export function isToolStepPending(step: ActivityStep): boolean {
  return step.kind === "tool" && !SETTLED_TOOL_STATES.has(step.state);
}

/** A failed step: errored or denied (both count as failures in the group line). */
export function isToolStepFailed(
  step: ActivityStep,
): step is Extract<ActivityStep, { kind: "tool" }> {
  return (
    step.kind === "tool" &&
    (step.state === "output-error" ||
      step.state === "output-denied" ||
      Boolean(step.errorText))
  );
}

/**
 * Counts a group's calls by kind. ACP reports one call as a tool part PLUS its
 * diff/terminal content parts (sharing `toolCallId`), so those fold into the
 * call they belong to instead of counting twice: a call with a diff is an edit,
 * one with a terminal is a command.
 */
export function activityCounts(
  steps: readonly ActivityStep[],
): Map<ToolKind, number> {
  const toolKinds = new Map<string, ToolKind>();
  const loose: ToolKind[] = [];
  for (const step of steps) {
    if (step.kind !== "tool") continue;
    const kind = toolKind(step);
    if (step.toolCallId) toolKinds.set(step.toolCallId, kind);
    else loose.push(kind);
  }
  for (const step of steps) {
    if (step.kind === "tool") continue;
    const kind: ToolKind = step.kind === "diff" ? "edit" : "command";
    const owner = step.toolCallId;
    if (owner && toolKinds.has(owner)) {
      if (kind === "edit" || toolKinds.get(owner) === "tool") {
        toolKinds.set(owner, kind);
      }
    } else {
      loose.push(kind);
    }
  }
  const counts = new Map<ToolKind, number>();
  for (const kind of [...toolKinds.values(), ...loose]) {
    counts.set(kind, (counts.get(kind) ?? 0) + 1);
  }
  return counts;
}

/** The one-line label for a step: its description, title, or command. */
export function stepLabel(step: ActivityStep): string | undefined {
  if (step.kind === "diff") return step.path;
  if (step.kind === "terminal") return undefined;
  const input =
    step.input && typeof step.input === "object"
      ? (step.input as Record<string, unknown>)
      : {};
  const description =
    typeof input.description === "string" && input.description.trim()
      ? input.description
      : undefined;
  return description ?? step.title ?? toolCommand(step.input) ?? step.name;
}

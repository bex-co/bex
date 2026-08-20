import { z } from "zod";
import type { UIMessage } from "ai";

// Classifies the driver's typed `data-acp-*` parts and the AI SDK tool parts
// into the Devin-shaped transcript groups the conversation column renders. The
// driver (`lego/agent-image/driver/src/acp-map.ts`) maps each ACP `session/update`
// straight into a typed UI-message chunk — text/reasoning/real dynamic tool parts
// plus these three typed data parts (no generic `data-acp` re-wrap to re-classify):
//   data-acp-plan     { entries: [{ content, status, priority }] }
//   data-acp-diff     { path, oldText, newText, toolCallId }
//   data-acp-terminal { terminalId, output?, toolCallId }
// so the browser switches on the part TYPE, not a discriminator inside an opaque
// payload. Any other transient `data-acp-*` part (available-commands, info) is
// ephemeral and renders nothing.

/**
 * The schema handed to `useChat`'s `dataPartSchemas` for each `data-acp-*` key.
 * Deliberately permissive: the verbatim-forward contract (ADR047 D3) means the
 * browser must render whatever the driver emitted, never reject it — validation
 * happens structurally in `classifyAcpPart`, not by dropping parts.
 */
export const acpDataSchema = z.unknown();

/** ISO-8601 UTC (`YYYY-MM-DDTHH:mm:ss.sssZ`) on a driver-emitted part. */
const ISO_UTC = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(\.\d{1,9})?Z$/;
const SOURCE_TIME_FIELD = "at";
const SOURCE_TIME_PROVIDER = "bex";

function utcMs(value: unknown): number | undefined {
  if (typeof value !== "string" || !ISO_UTC.test(value)) return undefined;
  const ms = Date.parse(value);
  return Number.isFinite(ms) ? ms : undefined;
}

export function isUtcTimestamp(value: unknown): value is string {
  return utcMs(value) !== undefined;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function bexAt(meta: unknown, field: "at" | "endAt"): unknown {
  return asRecord(asRecord(meta)?.[SOURCE_TIME_PROVIDER])?.[field];
}

/**
 * Distinct parsed source instants from a typed UI-message part. Looks at the
 * canonical top-level `at`, then `providerMetadata.bex.{at,endAt}` (text/
 * reasoning after useChat assembly) and the tool call/result metadata twins.
 * Duplicate echoes of the same instant collapse; invalid or missing optional
 * timing is skipped — never treated as transcript corruption.
 */
export function sourceTimestampsMs(
  part: Record<string, unknown>,
): number[] {
  const raw = [
    part[SOURCE_TIME_FIELD],
    bexAt(part.providerMetadata, "at"),
    bexAt(part.providerMetadata, "endAt"),
    bexAt(part.callProviderMetadata, "at"),
    bexAt(part.resultProviderMetadata, "at"),
  ];
  const out: number[] = [];
  const seen = new Set<number>();
  for (const value of raw) {
    const ms = utcMs(value);
    if (ms === undefined || seen.has(ms)) continue;
    seen.add(ms);
    out.push(ms);
  }
  return out;
}

export interface AcpPlanEntry {
  content: string;
  status?: string;
  priority?: string;
}

export type AcpGroup =
  | { kind: "plan"; entries: AcpPlanEntry[] }
  | { kind: "diff"; path?: string; oldText?: string; newText?: string }
  | { kind: "terminal"; terminalId?: string; output?: string };

function str(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

/** A string only if it has non-whitespace content, else undefined. */
function nonEmpty(value: string | undefined): string | undefined {
  return value && value.trim() ? value : undefined;
}

// Maps a `data-acp-plan` payload's entries onto the plan group.
function planGroup(data: unknown): AcpGroup {
  const record = asRecord(data);
  const rawEntries =
    record && Array.isArray(record.entries) ? record.entries : [];
  const entries: AcpPlanEntry[] = rawEntries.flatMap((entry) => {
    const e = asRecord(entry);
    const content = e ? str(e.content) : undefined;
    if (!content) return [];
    return [
      {
        content,
        status: e ? str(e.status) : undefined,
        priority: e ? str(e.priority) : undefined,
      },
    ];
  });
  return { kind: "plan", entries };
}

/**
 * Classifies a driver-emitted typed data part (`data-acp-plan|diff|terminal`)
 * onto its transcript group; returns undefined for any other part (text/tool
 * parts and transient `data-acp-*` parts are handled elsewhere / render nothing).
 */
export function classifyAcpPart(
  part: { type: string } & Record<string, unknown>,
): AcpGroup | undefined {
  const record = asRecord(part.data);
  switch (part.type) {
    case "data-acp-plan":
      return planGroup(part.data);
    case "data-acp-diff":
      return {
        kind: "diff",
        path: record ? str(record.path) : undefined,
        oldText: record ? str(record.oldText) : undefined,
        newText: record ? str(record.newText) : undefined,
      };
    case "data-acp-terminal":
      return {
        kind: "terminal",
        terminalId: record ? str(record.terminalId) : undefined,
        output: record
          ? (str(record.output) ?? str(record.stdout) ?? str(record.stderr))
          : undefined,
      };
    default:
      return undefined;
  }
}

/** True for the AI SDK tool parts (static `tool-<name>` or `dynamic-tool`). */
export function isToolPart(part: { type: string }): boolean {
  return part.type === "dynamic-tool" || part.type.startsWith("tool-");
}

export interface ToolPartInfo {
  name: string;
  state: string;
  input?: unknown;
  output?: unknown;
  errorText?: string;
}

/**
 * Extracts the renderable fields from a tool UI part without depending on the
 * exact `ToolUIPart`/`DynamicToolUIPart` union (the driver uses a dynamic tool).
 */
export function toolPartInfo(part: Record<string, unknown>): ToolPartInfo {
  const type = str(part.type) ?? "tool";
  const name =
    str(part.toolName) ??
    (type.startsWith("tool-") ? type.slice("tool-".length) : type);
  return {
    name,
    state: str(part.state) ?? "input-available",
    input: part.input,
    output: part.output,
    errorText: str(part.errorText),
  };
}

export interface UnwrappedTool {
  name: string;
  state: string;
  /** A shell command line, when the (unwrapped) input carries one. */
  command?: string;
  /** The tool arguments after unwrapping the ACP dynamic-tool envelope. */
  args?: unknown;
  /** The tool output, with trivial acks (`{ok:true}`) dropped as noise. */
  output?: unknown;
  errorText?: string;
}

/**
 * Unwraps the shipped provider's opaque ACP tool part into a human-renderable
 * shape. `@mcpc-tech/acp-ai-provider` collapses every ACP `tool_call` into ONE
 * dynamic tool named `acp.acp_provider_agent_dynamic_tool`, discarding the ACP
 * `title`/`kind` and stuffing the real call into `input = {toolCallId, toolName,
 * args}` with `output = rawOutput` — so a naive render dumps `{"command":"ls"}`
 * and `{"ok":true}` as raw JSON. This restores the real tool name from the
 * envelope, lifts a `command` up for shell-line rendering, and drops trivial
 * output acks. A non-enveloped tool part (e.g. the local mock's `acp_agent` with
 * a plain input) passes through unchanged.
 */
export function unwrapAcpTool(info: ToolPartInfo): UnwrappedTool {
  let name = info.name;
  let args = info.input;
  const envelope = asRecord(info.input);
  if (
    envelope &&
    typeof envelope.toolName === "string" &&
    "args" in envelope &&
    "toolCallId" in envelope
  ) {
    name = envelope.toolName;
    args = envelope.args;
  }
  const argRecord = asRecord(args);
  // First non-empty of command/commandLine (an empty `command` must not mask a
  // real `commandLine`, and a blank string is not a renderable command).
  const command = argRecord
    ? (nonEmpty(str(argRecord.command)) ?? nonEmpty(str(argRecord.commandLine)))
    : undefined;
  return {
    name,
    state: info.state,
    command,
    // Once a command is lifted out, the remaining arg object is just `{command}`
    // — don't also dump it as JSON.
    args: command ? undefined : args,
    output: isTrivialAck(info.output) ? undefined : info.output,
    errorText: info.errorText,
  };
}

/**
 * True for an output that carries no information a reader needs — a bare success
 * ack like `{ok:true}`, `{success:true}`, `true`, or an empty object. These are
 * the ACP `rawOutput` acks that otherwise render as noise `{"ok":true}` blobs.
 */
export function isTrivialAck(output: unknown): boolean {
  if (output === undefined || output === null || output === true) return true;
  const record = asRecord(output);
  if (!record) return false;
  const keys = Object.keys(record);
  if (keys.length === 0) return true;
  return keys.every(
    (k) =>
      (k === "ok" || k === "success" || k === "status") &&
      (record[k] === true || record[k] === "ok" || record[k] === "success"),
  );
}

/** The concrete UI message type the column renders (default parts + the typed
 * `data-acp-*` data parts the driver emits). */
export type AgentUIMessage = UIMessage<
  unknown,
  {
    "acp-plan": unknown;
    "acp-diff": unknown;
    "acp-terminal": unknown;
    "user-prompt": unknown;
  }
>;

import { z } from "zod";
import type { UIMessage } from "ai";

// Classifies the driver's `data-acp` parts and the AI SDK tool parts into the
// Devin-shaped transcript groups the conversation column renders. The wire
// shapes come from the shipped driver: the AI SDK provider maps ACP
// `session/update` notifications into `raw` chunks the driver re-wraps as
// `data-acp` parts (`lego/agent-image/driver/src/session.ts` → `stream-hub.ts`),
// while text/reasoning/tool activity arrives as standard UI-message parts.
//
// The `data-acp` payload is one of the provider's raw values:
//   { type: "plan", entries: [{ content, status, priority }] }
//   { type: "diff", path, oldText, newText, toolCallId }
//   { type: "terminal", terminalId, output?, toolCallId }
// The session-log/evidence form additionally uses a `sessionUpdate` discriminator
// ({ sessionUpdate: "tool_call", title, kind, command }); both are tolerated so
// the column never blanks on a shape variant.

/**
 * The schema handed to `useChat`'s `dataPartSchemas` for the `acp` data key
 * (i.e. `data-acp` parts). Deliberately permissive: the verbatim-forward
 * contract (ADR047 D3) means the browser must render whatever the driver
 * emitted, never reject it — validation happens structurally in
 * `classifyAcpData`, not by dropping parts.
 */
export const acpDataSchema = z.unknown();

export interface AcpPlanEntry {
  content: string;
  status?: string;
  priority?: string;
}

export type AcpGroup =
  | { kind: "plan"; entries: AcpPlanEntry[] }
  | { kind: "diff"; path?: string; oldText?: string; newText?: string }
  | { kind: "terminal"; terminalId?: string; output?: string }
  | { kind: "command"; title?: string; command?: string; toolKind?: string }
  | { kind: "unknown"; data: unknown };

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function str(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

/** Maps one `data-acp` payload onto its transcript group. */
export function classifyAcpData(data: unknown): AcpGroup {
  const record = asRecord(data);
  if (!record) return { kind: "unknown", data };

  const discriminator = str(record.type) ?? str(record.sessionUpdate);
  switch (discriminator) {
    case "plan": {
      const rawEntries = Array.isArray(record.entries) ? record.entries : [];
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
    case "diff":
      return {
        kind: "diff",
        path: str(record.path),
        oldText: str(record.oldText),
        newText: str(record.newText),
      };
    case "terminal":
      return {
        kind: "terminal",
        terminalId: str(record.terminalId),
        output: str(record.output) ?? str(record.stdout) ?? str(record.stderr),
      };
    case "tool_call":
    case "tool_call_update":
      return {
        kind: "command",
        title: str(record.title),
        command: str(record.command) ?? str(record.commandLine),
        toolKind: str(record.kind),
      };
    default:
      return { kind: "unknown", data };
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

/** Narrows a message part to a `data-acp` part and returns its payload. */
export function acpPartData(
  part: { type: string } & Record<string, unknown>,
): unknown | undefined {
  return part.type === "data-acp" ? part.data : undefined;
}

/** The concrete UI message type the column renders (default parts + `data-acp`). */
export type AgentUIMessage = UIMessage<unknown, { acp: unknown }>;

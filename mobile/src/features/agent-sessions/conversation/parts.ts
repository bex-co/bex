export const MAX_RENDERED_PART_CHARS = 20_000;
export const COLLAPSED_PART_CHARS = 1_200;

export type PartLike = { type: string } & Record<string, unknown>;

export type PlanEntry = {
  content: string;
  status?: string;
  priority?: string;
};

export type RenderablePart =
  | { kind: "text"; text: string }
  | { kind: "reasoning"; text: string }
  | { kind: "plan"; entries: PlanEntry[] }
  | { kind: "diff"; path?: string; text: string }
  | { kind: "terminal"; terminalId?: string; text: string }
  | {
      kind: "tool";
      name: string;
      state: string;
      input?: string;
      output?: string;
      error?: string;
    }
  | { kind: "unknown"; type: string };

export function classifyConversationPart(part: PartLike): RenderablePart {
  if (part.type === "text") {
    return { kind: "text", text: capText(asString(part.text) ?? "") };
  }
  if (part.type === "reasoning") {
    return { kind: "reasoning", text: capText(asString(part.text) ?? "") };
  }
  if (part.type === "data-acp-plan") {
    return { kind: "plan", entries: planEntries(part.data) };
  }
  if (part.type === "data-acp-diff") {
    const data = asRecord(part.data);
    return {
      kind: "diff",
      path: asString(data?.path),
      text: capText(
        unifiedDiff(asString(data?.oldText), asString(data?.newText)),
      ),
    };
  }
  if (part.type === "data-acp-terminal") {
    const data = asRecord(part.data);
    return {
      kind: "terminal",
      terminalId: asString(data?.terminalId),
      text: capText(
        asString(data?.output) ??
          asString(data?.stdout) ??
          asString(data?.stderr) ??
          "",
      ),
    };
  }
  if (part.type === "dynamic-tool" || part.type.startsWith("tool-")) {
    return toolPart(part);
  }
  return { kind: "unknown", type: part.type };
}

function toolPart(part: PartLike): RenderablePart {
  const envelope = asRecord(part.input);
  const hasEnvelope =
    envelope &&
    typeof envelope.toolName === "string" &&
    "args" in envelope &&
    "toolCallId" in envelope;
  const name =
    (hasEnvelope ? asString(envelope.toolName) : undefined) ??
    asString(part.toolName) ??
    (part.type.startsWith("tool-") ? part.type.slice(5) : part.type);
  const input = hasEnvelope ? envelope.args : part.input;
  return {
    kind: "tool",
    name,
    state: asString(part.state) ?? "input-available",
    input: input === undefined ? undefined : capText(safeJson(input)),
    output: isTrivialAck(part.output)
      ? undefined
      : capText(safeJson(part.output)),
    error: asString(part.errorText),
  };
}

function planEntries(value: unknown): PlanEntry[] {
  const data = asRecord(value);
  if (!Array.isArray(data?.entries)) return [];
  return data.entries.flatMap((raw) => {
    const entry = asRecord(raw);
    const content = asString(entry?.content);
    if (!content?.trim()) return [];
    return [
      {
        content: capText(content),
        status: asString(entry?.status),
        priority: asString(entry?.priority),
      },
    ];
  });
}

export function unifiedDiff(oldText = "", newText = ""): string {
  if (!oldText && !newText) return "";
  const before = oldText.split("\n");
  const after = newText.split("\n");
  return [
    "--- before",
    "+++ after",
    ...before.map((line) => `-${line}`),
    ...after.map((line) => `+${line}`),
  ].join("\n");
}

export function safeJson(value: unknown): string {
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2) ?? String(value);
  } catch {
    return "[unavailable]";
  }
}

export function isTrivialAck(value: unknown): boolean {
  if (value === undefined || value === null || value === true) return true;
  const record = asRecord(value);
  if (!record) return false;
  const keys = Object.keys(record);
  if (keys.length === 0) return true;
  return keys.every(
    (key) =>
      (key === "ok" || key === "success" || key === "status") &&
      (record[key] === true ||
        record[key] === "ok" ||
        record[key] === "success"),
  );
}

function capText(value: string): string {
  return value.length <= MAX_RENDERED_PART_CHARS
    ? value
    : `${value.slice(0, MAX_RENDERED_PART_CHARS)}\n[…]`;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function asString(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

import { describe, expect, it } from "vitest";
import {
  classifyAcpPart,
  isUtcTimestamp,
  sourceTimestampsMs,
  toolCommand,
  toolPartInfo,
} from "@/features/agent-sessions/lib/acp-parts";

describe("classifyAcpPart", () => {
  it("classifies the driver's typed data parts by part type", () => {
    const plan = classifyAcpPart({
      type: "data-acp-plan",
      data: {
        entries: [{ content: "step", status: "in_progress", priority: "high" }],
      },
    });
    expect(plan).toEqual({
      kind: "plan",
      entries: [{ content: "step", status: "in_progress", priority: "high" }],
    });

    const diff = classifyAcpPart({
      type: "data-acp-diff",
      data: { path: "a.txt", oldText: "", newText: "x\n" },
    });
    expect(diff).toEqual({
      kind: "diff",
      path: "a.txt",
      oldText: "",
      newText: "x\n",
    });

    const terminal = classifyAcpPart({
      type: "data-acp-terminal",
      data: { terminalId: "t1", output: "done" },
    });
    expect(terminal).toEqual({
      kind: "terminal",
      terminalId: "t1",
      output: "done",
    });
  });

  it("drops plan entries without content", () => {
    const plan = classifyAcpPart({
      type: "data-acp-plan",
      data: { entries: [{ status: "pending" }, { content: "real" }] },
    });
    expect(plan).toEqual({
      kind: "plan",
      entries: [{ content: "real", status: undefined, priority: undefined }],
    });
  });

  it("returns undefined for non-acp / transient parts", () => {
    expect(classifyAcpPart({ type: "text", text: "hi" })).toBeUndefined();
    expect(
      classifyAcpPart({ type: "dynamic-tool", toolName: "bash" }),
    ).toBeUndefined();
    expect(
      classifyAcpPart({ type: "data-acp-available-commands", data: {} }),
    ).toBeUndefined();
  });

  it("classifies timestamped parts without dropping payload", () => {
    const plan = classifyAcpPart({
      type: "data-acp-plan",
      at: "2026-08-19T00:00:00.000Z",
      data: { entries: [{ content: "step" }] },
    });
    expect(plan).toEqual({
      kind: "plan",
      entries: [{ content: "step", status: undefined, priority: undefined }],
    });
  });
});

describe("sourceTimestampsMs", () => {
  it("exposes a valid top-level at", () => {
    expect(
      sourceTimestampsMs({
        type: "data-acp-diff",
        at: "2026-08-19T00:00:40.000Z",
      }),
    ).toEqual([Date.parse("2026-08-19T00:00:40.000Z")]);
  });

  it("treats missing and invalid optional timing as unavailable", () => {
    expect(sourceTimestampsMs({ type: "text", text: "hi" })).toEqual([]);
    expect(
      sourceTimestampsMs({ type: "text", at: "not-a-time", text: "hi" }),
    ).toEqual([]);
    expect(sourceTimestampsMs({ type: "text", at: 12, text: "hi" })).toEqual(
      [],
    );
    expect(isUtcTimestamp("2026-08-19T00:00:00.000Z")).toBe(true);
    expect(isUtcTimestamp("2026-08-19 00:00:00")).toBe(false);
  });

  it("reads assembled providerMetadata and tool metadata twins", () => {
    expect(
      sourceTimestampsMs({
        type: "reasoning",
        at: "2026-08-19T00:00:00.000Z",
        providerMetadata: {
          bex: {
            at: "2026-08-19T00:00:00.000Z",
            endAt: "2026-08-19T00:00:12.000Z",
          },
        },
      }),
    ).toEqual([
      Date.parse("2026-08-19T00:00:00.000Z"),
      Date.parse("2026-08-19T00:00:12.000Z"),
    ]);
    expect(
      sourceTimestampsMs({
        type: "dynamic-tool",
        callProviderMetadata: { bex: { at: "2026-08-19T00:00:00.000Z" } },
        resultProviderMetadata: { bex: { at: "2026-08-19T00:00:40.000Z" } },
      }),
    ).toEqual([
      Date.parse("2026-08-19T00:00:00.000Z"),
      Date.parse("2026-08-19T00:00:40.000Z"),
    ]);
  });
});

describe("native tool parts", () => {
  it("preserves the driver's title, arguments, output, and errors", () => {
    expect(
      toolPartInfo({
        type: "dynamic-tool",
        toolName: "Search repository",
        state: "output-error",
        input: { q: "x" },
        output: { hits: 2 },
        errorText: "boom",
      }),
    ).toEqual({
      name: "Search repository",
      state: "output-error",
      input: { q: "x" },
      output: { hits: 2 },
      errorText: "boom",
    });
  });

  it("extracts shell commands and ignores blank or non-command inputs", () => {
    expect(toolCommand({ command: "ls" })).toBe("ls");
    expect(toolCommand({ command: "  ", commandLine: "echo hi" })).toBe(
      "echo hi",
    );
    expect(toolCommand({ command: "" })).toBeUndefined();
    expect(toolCommand({ q: "x" })).toBeUndefined();
    expect(toolCommand(null)).toBeUndefined();
  });
});

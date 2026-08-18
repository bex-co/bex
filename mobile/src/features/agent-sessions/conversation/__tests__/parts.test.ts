import { classifyConversationPart, isTrivialAck, safeJson } from "../parts";

describe("native agent conversation parts", () => {
  it("classifies ACP plan, diff, terminal, and wrapped tools", () => {
    expect(
      classifyConversationPart({
        type: "data-acp-plan",
        data: { entries: [{ content: "Run tests", status: "completed" }] },
      }),
    ).toEqual({
      kind: "plan",
      entries: [
        { content: "Run tests", status: "completed", priority: undefined },
      ],
    });
    expect(
      classifyConversationPart({
        type: "data-acp-diff",
        data: { path: "app.ts", oldText: "old", newText: "new" },
      }),
    ).toEqual({
      kind: "diff",
      path: "app.ts",
      text: "--- before\n+++ after\n-old\n+new",
    });
    expect(
      classifyConversationPart({
        type: "data-acp-terminal",
        data: { terminalId: "term-1", stdout: "tests pass" },
      }),
    ).toEqual({ kind: "terminal", terminalId: "term-1", text: "tests pass" });
    expect(
      classifyConversationPart({
        type: "dynamic-tool",
        state: "output-available",
        toolName: "acp-wrapper",
        input: {
          toolCallId: "t1",
          toolName: "shell",
          args: { command: "go test ./..." },
        },
        output: { ok: true },
      }),
    ).toEqual({
      kind: "tool",
      name: "shell",
      state: "output-available",
      input: '{\n  "command": "go test ./..."\n}',
      output: undefined,
      error: undefined,
    });
  });

  it("falls back for unknown parts without dumping their untrusted payload", () => {
    expect(
      classifyConversationPart({
        type: "data-future-part",
        data: { secret: "do not render raw" },
      }),
    ).toEqual({ kind: "unknown", type: "data-future-part" });
  });

  it("serializes safely and drops trivial tool acknowledgments", () => {
    const circular: Record<string, unknown> = {};
    circular.self = circular;
    expect(safeJson(circular)).toBe("[unavailable]");
    expect(isTrivialAck({ ok: true })).toBe(true);
    expect(isTrivialAck({ result: "changed" })).toBe(false);
  });
});

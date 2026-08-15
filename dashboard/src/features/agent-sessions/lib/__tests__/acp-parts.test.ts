import { describe, expect, it } from "vitest";
import {
  classifyAcpPart,
  isTrivialAck,
  unwrapAcpTool,
  type ToolPartInfo,
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
});

const info = (over: Partial<ToolPartInfo>): ToolPartInfo => ({
  name: "tool",
  state: "output-available",
  ...over,
});

describe("unwrapAcpTool", () => {
  it("unwraps the ACP dynamic-tool envelope to the real name + command", () => {
    // The shipped provider collapses every ACP tool into one dynamic tool whose
    // input is {toolCallId, toolName, args}; the raw dump was `{"command":"ls"}`.
    const t = unwrapAcpTool(
      info({
        name: "acp.acp_provider_agent_dynamic_tool",
        input: { toolCallId: "c1", toolName: "bash", args: { command: "ls" } },
        output: { ok: true },
      }),
    );
    expect(t.name).toBe("bash"); // real name recovered, not the opaque wrapper
    expect(t.command).toBe("ls"); // command lifted for shell-line rendering
    expect(t.args).toBeUndefined(); // the {command} arg is not also dumped as JSON
    expect(t.output).toBeUndefined(); // trivial {ok:true} ack dropped as noise
  });

  it("lifts a command from a plain (non-enveloped) tool input", () => {
    const t = unwrapAcpTool(
      info({ name: "acp_agent", input: { command: "ls" } }),
    );
    expect(t.name).toBe("acp_agent");
    expect(t.command).toBe("ls");
    expect(t.args).toBeUndefined();
  });

  it("keeps non-command args and non-trivial output", () => {
    const t = unwrapAcpTool(
      info({ name: "search", input: { q: "x" }, output: { hits: 2 } }),
    );
    expect(t.name).toBe("search");
    expect(t.command).toBeUndefined();
    expect(t.args).toEqual({ q: "x" });
    expect(t.output).toEqual({ hits: 2 });
  });

  it("ignores a blank command and falls back to commandLine", () => {
    const t = unwrapAcpTool(
      info({ name: "bash", input: { command: "  ", commandLine: "echo hi" } }),
    );
    expect(t.command).toBe("echo hi"); // empty `command` must not mask commandLine
  });

  it("does not treat a blank command as a command", () => {
    const t = unwrapAcpTool(info({ name: "bash", input: { command: "" } }));
    expect(t.command).toBeUndefined();
    expect(t.args).toEqual({ command: "" }); // falls back to showing the args
  });

  it("passes an error through", () => {
    const t = unwrapAcpTool(
      info({ name: "bash", state: "output-error", errorText: "boom" }),
    );
    expect(t.errorText).toBe("boom");
  });
});

describe("isTrivialAck", () => {
  it("treats bare success acks as trivial", () => {
    expect(isTrivialAck({ ok: true })).toBe(true);
    expect(isTrivialAck({ success: true })).toBe(true);
    expect(isTrivialAck({ status: "ok" })).toBe(true);
    expect(isTrivialAck({})).toBe(true);
    expect(isTrivialAck(true)).toBe(true);
    expect(isTrivialAck(null)).toBe(true);
    expect(isTrivialAck(undefined)).toBe(true);
  });

  it("treats informative output as non-trivial", () => {
    expect(isTrivialAck({ hits: 2 })).toBe(false);
    expect(isTrivialAck({ ok: false })).toBe(false);
    expect(isTrivialAck("some text")).toBe(false);
    expect(isTrivialAck({ ok: true, extra: 1 })).toBe(false);
  });
});

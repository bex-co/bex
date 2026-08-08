import { describe, expect, it } from "vitest";
import {
  isTrivialAck,
  unwrapAcpTool,
  type ToolPartInfo,
} from "@/features/agent-sessions/lib/acp-parts";

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

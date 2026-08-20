import { describe, it, expect } from "vitest";
import {
  logServerError,
  writeServerLog,
  type ServerLogWriter,
} from "../server-log";
import { withReportedRouteError } from "../report-route-error";

function capture(): { lines: string[]; out: ServerLogWriter } {
  const lines: string[] = [];
  return {
    lines,
    out: {
      write(chunk: string) {
        lines.push(chunk);
      },
    },
  };
}

describe("writeServerLog (w4/m88)", () => {
  it("emits one JSON line with level and msg", () => {
    const { lines, out } = capture();
    writeServerLog({ level: "error", msg: "boom" }, out);
    expect(lines).toHaveLength(1);
    expect(lines[0].endsWith("\n")).toBe(true);
    expect(JSON.parse(lines[0]!)).toEqual({ level: "error", msg: "boom" });
  });

  it("includes path and status when provided", () => {
    const { lines, out } = capture();
    logServerError(
      { msg: "ssr_failed", path: "/auth/consent", status: 500 },
      out,
    );
    expect(JSON.parse(lines[0]!)).toEqual({
      level: "error",
      msg: "ssr_failed",
      path: "/auth/consent",
      status: 500,
    });
  });

  it("truncates oversized msg and path", () => {
    const { lines, out } = capture();
    writeServerLog(
      {
        level: "error",
        msg: "m".repeat(600),
        path: "/".padEnd(300, "p"),
      },
      out,
    );
    const parsed = JSON.parse(lines[0]!) as { msg: string; path: string };
    expect(parsed.msg.length).toBe(512);
    expect(parsed.msg.endsWith("…")).toBe(true);
    expect(parsed.path.length).toBe(256);
    expect(parsed.path.endsWith("…")).toBe(true);
  });

  it("emits only level/msg/path/status keys (no secret-bearing fields)", () => {
    const { lines, out } = capture();
    writeServerLog(
      { level: "error", msg: "x", path: "/y", status: 500 },
      out,
    );
    const keys = Object.keys(JSON.parse(lines[0]!)).sort();
    expect(keys).toEqual(["level", "msg", "path", "status"]);
  });
});

describe("withReportedRouteError (w4/m88)", () => {
  it("rethrows after reporting", async () => {
    await expect(
      withReportedRouteError(async () => {
        throw new Error("consent_boom");
      }),
    ).rejects.toThrow("consent_boom");
  });

  it("returns the fn result on success without throwing", async () => {
    await expect(withReportedRouteError(async () => 42)).resolves.toBe(42);
  });
});

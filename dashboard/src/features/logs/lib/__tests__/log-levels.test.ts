import { describe, expect, it } from "vitest";
import { lineLevel, lineLevelText } from "../log-levels";

const app = (level: string) => ({ level, type: "app", statusCode: "" });
const request = (statusCode: string, level = "") => ({
  level,
  type: "request",
  statusCode,
});

describe("lineLevel (w3/m509)", () => {
  it("marks each of Render's eight levels", () => {
    expect(
      [
        "debug",
        "info",
        "notice",
        "warning",
        "error",
        "critical",
        "alert",
        "emergency",
      ].map((level) => lineLevel(app(level))),
    ).toEqual([
      "debug",
      "info",
      "notice",
      "warning",
      "error",
      "error",
      "error",
      "error",
    ]);
  });

  it("reads the spellings loggers use for the same levels, in any case", () => {
    expect(lineLevel(app("WARN"))).toBe("warning");
    expect(lineLevel(app("fatal"))).toBe("error");
    expect(lineLevel(app("Trace"))).toBe("debug");
  });

  it("marks nothing for a line that names no level it knows", () => {
    expect(lineLevel(app(""))).toBeNull();
    expect(lineLevel(app("verbose"))).toBeNull();
  });

  it("gives a request line with no level label its status class's level", () => {
    expect(lineLevel(request("404"))).toBe("warning");
    expect(lineLevel(request("503"))).toBe("error");
    expect(lineLevel(request("200"))).toBe("info");
    expect(lineLevel(request("301"))).toBe("info");
    expect(lineLevel(request(""))).toBeNull();
  });

  it("keeps a request line's own level label over its status", () => {
    expect(lineLevel(request("404", "error"))).toBe("error");
  });

  it("only derives a level from the status of a request line", () => {
    expect(lineLevel({ level: "", type: "app", statusCode: "500" })).toBeNull();
  });
});

describe("lineLevelText", () => {
  it("names the line's own level, not the mark it shares", () => {
    expect(lineLevelText({ level: "CRITICAL" }, "error")).toBe("critical");
    expect(lineLevelText({ level: "" }, "warning")).toBe("warning");
  });
});

import { describe, it, expect } from "vitest";
import {
  liveTailUnsupported,
  EMPTY_LOG_FILTERS,
  type LogFilters,
} from "../types";

// w8/m147: the viewer's live gate mirrors bex-api's subscribe routing —
// request filters stream request lines, a level streams app and request lines
// (w4/584), and only a type that cannot carry a filter keeps the tail paused.
describe("liveTailUnsupported", () => {
  const f = (over: Partial<LogFilters>): LogFilters => ({
    ...EMPTY_LOG_FILTERS,
    ...over,
  });

  it("serves request views with request filters", () => {
    expect(liveTailUnsupported(f({ type: "request" }))).toBe(false);
    expect(
      liveTailUnsupported(
        f({ type: "request", statusCode: "500", method: "GET", path: "/x" }),
      ),
    ).toBe(false);
  });

  it("serves unfiltered app and all views", () => {
    expect(liveTailUnsupported(f({ type: "app" }))).toBe(false);
    expect(liveTailUnsupported(f({ type: "all", instance: "bv1" }))).toBe(
      false,
    );
  });

  it("serves structured filters on the all view and level on the app view", () => {
    expect(liveTailUnsupported(f({ type: "all", statusCode: "500" }))).toBe(
      false,
    );
    expect(liveTailUnsupported(f({ type: "all", path: "/x" }))).toBe(false);
    expect(liveTailUnsupported(f({ type: "all", level: "error" }))).toBe(false);
    expect(
      liveTailUnsupported(f({ type: "all", level: "error", method: "GET" })),
    ).toBe(false);
    expect(liveTailUnsupported(f({ type: "app", level: "error" }))).toBe(false);
    expect(liveTailUnsupported(f({ type: "request", level: "error" }))).toBe(
      false,
    );
  });

  it("pauses when the type cannot carry a filter", () => {
    expect(liveTailUnsupported(f({ type: "app", statusCode: "500" }))).toBe(
      true,
    );
    expect(liveTailUnsupported(f({ type: "app", method: "GET" }))).toBe(true);
    expect(liveTailUnsupported(f({ type: "app", path: "/x" }))).toBe(true);
    expect(liveTailUnsupported(f({ type: "app", host: "a.onbex.co" }))).toBe(
      true,
    );
  });
});

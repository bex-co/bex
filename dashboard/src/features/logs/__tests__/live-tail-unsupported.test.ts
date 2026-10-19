import { describe, it, expect } from "vitest";
import {
  liveTailUnsupported,
  EMPTY_LOG_FILTERS,
  type LogFilters,
} from "../types";

// w4/m237: the viewer's live gate mirrors bex-api's subscribe acceptance —
// request views (with any structured filters) stream from the store poll;
// store-only filters over app lines keep the tail paused.
describe("liveTailUnsupported", () => {
  const f = (over: Partial<LogFilters>): LogFilters => ({
    ...EMPTY_LOG_FILTERS,
    ...over,
  });

  it("serves request views with or without structured filters", () => {
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

  it("pauses for store-only filters over app lines", () => {
    expect(liveTailUnsupported(f({ type: "all", statusCode: "500" }))).toBe(
      true,
    );
    expect(liveTailUnsupported(f({ type: "app", level: "error" }))).toBe(true);
    expect(liveTailUnsupported(f({ type: "all", path: "/x" }))).toBe(true);
  });
});

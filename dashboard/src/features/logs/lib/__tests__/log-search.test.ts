import { describe, expect, it } from "vitest";
import {
  logFiltersFromSearch,
  logFiltersToSearch,
  logRangeFromSearch,
  logSearchEquals,
  parseLogSearch,
} from "../log-search";
import { EMPTY_LOG_FILTERS, type LogFilters } from "../../types";

describe("log range URL state", () => {
  it("round-trips a supported selection through route validation", () => {
    const restored = parseLogSearch({ range: "12h" });
    expect(restored).toEqual({ range: "12h" });
    expect(logRangeFromSearch(restored).id).toBe("12h");
  });

  it("drops malformed values and restores the one-hour default", () => {
    const restored = parseLogSearch({ range: "31d" });
    expect(restored).toEqual({});
    expect(logRangeFromSearch(restored).id).toBe("1h");
  });
});

describe("log filter URL state (w7/m42)", () => {
  it("round-trips the full filter set through parse → filters → serialize", () => {
    const search = parseLogSearch({
      range: "4h",
      type: "request",
      level: "error",
      method: "GET",
      statusCode: "5xx",
      instance: "web-abc12",
      path: "/api",
      text: "boom",
      live: 0,
    });
    expect(search).toEqual({
      range: "4h",
      type: "request",
      level: "error",
      method: "GET",
      statusCode: "5xx",
      instance: "web-abc12",
      path: "/api",
      text: "boom",
      live: 0,
    });

    const filters = logFiltersFromSearch(search);
    expect(filters).toEqual({
      type: "request",
      level: "error",
      method: "GET",
      statusCode: "5xx",
      instance: "web-abc12",
      path: "/api",
      text: "boom",
    } satisfies LogFilters);

    // Serializing the restored state reproduces the same search keys.
    expect(logFiltersToSearch(filters, false)).toEqual({
      type: "request",
      level: "error",
      method: "GET",
      statusCode: "5xx",
      instance: "web-abc12",
      path: "/api",
      text: "boom",
      live: 0,
    });
  });

  it("omits defaults so an unfiltered view keeps a clean URL", () => {
    expect(logFiltersToSearch(EMPTY_LOG_FILTERS, true)).toEqual({});
  });

  it("serializes only the filters that are set", () => {
    expect(
      logFiltersToSearch({ ...EMPTY_LOG_FILTERS, level: "error" }, true),
    ).toEqual({ level: "error" });
  });

  it("accepts a numeric statusCode (TanStack JSON-parses ?statusCode=404)", () => {
    expect(parseLogSearch({ statusCode: 404 })).toEqual({ statusCode: "404" });
  });

  it("drops a malformed type without crashing", () => {
    const search = parseLogSearch({ type: "bogus" });
    expect(search).toEqual({});
    expect(logFiltersFromSearch(search).type).toBe("all");
  });

  it("treats live=0 / false as off and everything else as on (omitted)", () => {
    expect(parseLogSearch({ live: 0 })).toEqual({ live: 0 });
    expect(parseLogSearch({ live: "0" })).toEqual({ live: 0 });
    expect(parseLogSearch({ live: false })).toEqual({ live: 0 });
    expect(parseLogSearch({ live: 1 })).toEqual({});
    expect(parseLogSearch({ live: true })).toEqual({});
  });
});

describe("Render deep-link aliases (w7/m42/t003)", () => {
  it("prefills from Render's t/r keys", () => {
    expect(parseLogSearch({ t: "app", r: "1h" })).toEqual({
      type: "app",
      range: "1h",
    });
  });

  it("honors Render's `application` type alias", () => {
    expect(parseLogSearch({ t: "application" })).toEqual({ type: "app" });
    expect(parseLogSearch({ type: "application" })).toEqual({ type: "app" });
  });

  it("canonical keys win over aliases when both are present", () => {
    expect(parseLogSearch({ type: "app", t: "request" })).toEqual({
      type: "app",
    });
    expect(parseLogSearch({ range: "4h", r: "1h" })).toEqual({ range: "4h" });
  });

  it("maps Render's own r grammar (15m|6h) onto the nearest bex preset; 24h/7d now parse directly (w5/m42)", () => {
    expect(parseLogSearch({ r: "15m" })).toEqual({ range: "30m" });
    expect(parseLogSearch({ r: "6h" })).toEqual({ range: "4h" });
    expect(parseLogSearch({ r: "24h" })).toEqual({ range: "24h" });
    expect(parseLogSearch({ r: "7d" })).toEqual({ range: "7d" });
  });

  it("falls back to defaults for an r value bex has no preset for", () => {
    const search = parseLogSearch({ r: "45m" });
    expect(search).toEqual({});
    expect(logRangeFromSearch(search).id).toBe("1h");
  });

  it("degrades a retired preset id (pre-w5/m42 bookmark) to the default range", () => {
    for (const retired of ["3h", "6h", "1d"]) {
      const search = parseLogSearch({ range: retired });
      expect(search.range).toBeUndefined();
      expect(logRangeFromSearch(search).id).toBe("1h");
    }
  });

  it("never re-emits alias keys: the serializer writes canonical keys only", () => {
    const search = parseLogSearch({ t: "request", r: "6h" });
    const out = logFiltersToSearch(logFiltersFromSearch(search), true);
    expect(out).toEqual({ type: "request" });
    expect("t" in out).toBe(false);
    expect("r" in out).toBe(false);
  });
});

describe("logSearchEquals (the route's skip-when-unchanged guard)", () => {
  it("treats a round-tripped state as equal so the mount sync never navigates", () => {
    const search = parseLogSearch({
      range: "4h",
      type: "app",
      level: "error",
      live: 0,
    });
    const next = {
      range: search.range,
      ...logFiltersToSearch(logFiltersFromSearch(search), search.live !== 0),
    };
    expect(logSearchEquals(search, next)).toBe(true);
  });

  it("detects a real change on any filter key", () => {
    expect(logSearchEquals({ level: "error" }, { level: "warn" })).toBe(false);
    expect(logSearchEquals({}, { live: 0 })).toBe(false);
    expect(logSearchEquals({ range: "1h" }, { range: "6h" })).toBe(false);
  });
});

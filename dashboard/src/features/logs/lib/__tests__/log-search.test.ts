import { describe, expect, it } from "vitest";
import {
  CLEARED_LOG_SEARCH,
  logFiltersFromSearch,
  logFiltersToSearch,
  logRangeFromSearch,
  logContextSearch,
  logSearchEquals,
  parseLogSearch,
  serviceLineShell,
  serviceLogContextHref,
} from "../log-search";
import { datastoreLogContextHref } from "../datastore-log-search";
import { projectLogContextHref } from "../project-log-search";
import { SavedSearchType } from "@/graphql/definitions";
import { EMPTY_LOG_FILTERS, type LogFilters, type LogLine } from "../../types";

describe("log range URL state", () => {
  it("round-trips a supported selection through route validation", () => {
    const restored = parseLogSearch({ range: "12h" });
    expect(restored).toEqual({ range: "12h" });
    expect(logRangeFromSearch(restored).id).toBe("12h");
  });

  it("rejects malformed values and restores the one-hour default", () => {
    const restored = parseLogSearch({ range: "31d" });
    expect(restored).toStrictEqual(CLEARED_LOG_SEARCH);
    expect(logRangeFromSearch(restored).id).toBe("1h");
  });

  it("round-trips a custom absolute range (w5/m56)", () => {
    const restored = parseLogSearch({
      range: "custom",
      rangeStart: "2026-07-01T00:00:00.000Z",
      rangeEnd: "2026-07-01T06:00:00.000Z",
    });
    expect(restored).toEqual({
      range: "custom",
      rangeStart: "2026-07-01T00:00:00.000Z",
      rangeEnd: "2026-07-01T06:00:00.000Z",
    });
    const range = logRangeFromSearch(restored);
    expect(range.id).toBe("custom");
    expect(range).toMatchObject({
      startTime: "2026-07-01T00:00:00.000Z",
      endTime: "2026-07-01T06:00:00.000Z",
    });
  });

  it("ignores a custom range whose bounds are missing or reversed", () => {
    // reversed bounds don't validate → falls back to the default
    expect(
      logRangeFromSearch(
        parseLogSearch({
          range: "custom",
          rangeStart: "2026-07-02T00:00:00Z",
          rangeEnd: "2026-07-01T00:00:00Z",
        }),
      ).id,
    ).toBe("1h");
    // range=custom with no bounds is rejected entirely
    expect(parseLogSearch({ range: "custom" })).toStrictEqual(
      CLEARED_LOG_SEARCH,
    );
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
      host: "",
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

  it("rejects a malformed type without crashing", () => {
    const search = parseLogSearch({ type: "bogus" });
    expect(search).toStrictEqual(CLEARED_LOG_SEARCH);
    expect(logFiltersFromSearch(search).type).toBe("all");
  });

  it("treats live=0 / false as off and everything else as on (undefined)", () => {
    expect(parseLogSearch({ live: 0 })).toEqual({ live: 0 });
    expect(parseLogSearch({ live: "0" })).toEqual({ live: 0 });
    expect(parseLogSearch({ live: false })).toEqual({ live: 0 });
    expect(parseLogSearch({ live: 1 })).toStrictEqual(CLEARED_LOG_SEARCH);
    expect(parseLogSearch({ live: true })).toStrictEqual(CLEARED_LOG_SEARCH);
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
    expect(search).toStrictEqual(CLEARED_LOG_SEARCH);
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
    expect(logSearchEquals({ range: "1h" }, { range: "4h" })).toBe(false);
  });
});

// View in context (w3/m528): Render's link is the same explorer with its
// default filters, `contextLogId` the line and the range ±10 minutes.
describe("View in context links (w3/m528)", () => {
  const at = "2026-11-20T10:30:15.250Z";
  const line = (over: Partial<LogLine> = {}): LogLine => ({
    key: "id:abc",
    timestamp: at,
    instance: "srv-a-0123456789abcdefghij",
    message: "boom",
    spans: null,
    resource: "srv-a",
    type: "app",
    level: "error",
    method: "",
    statusCode: "",
    host: "",
    path: "",
    ...over,
  });
  const query = (href: string) => {
    const url = new URL(href, "https://dashboard.bex.co");
    return { path: url.pathname, search: Object.fromEntries(url.searchParams) };
  };

  it("opens the service Logs tab unfiltered, paused, ten minutes either side of the line, the line marked", () => {
    const href = serviceLogContextHref("srv-a", line())!;
    const { path, search } = query(href);
    expect(path).toBe("/services/srv-a/logs");
    expect(search).toEqual({
      range: "custom",
      rangeStart: "2026-11-20T10:20:15.250Z",
      rangeEnd: "2026-11-20T10:40:15.250Z",
      live: "0",
      contextLine: "id:abc",
    });
    // The route reads back exactly that state, with no filter.
    const parsed = parseLogSearch(
      Object.fromEntries(
        Object.entries(search).map(([k, v]) => [k, k === "live" ? 0 : v]),
      ),
    );
    expect(logFiltersFromSearch(parsed)).toEqual(EMPTY_LOG_FILTERS);
    expect(parsed.contextLine).toBe("id:abc");
    expect(logRangeFromSearch(parsed).id).toBe("custom");
  });

  it("opens a Postgres or Key Value Logs tab the same way", () => {
    expect(
      query(datastoreLogContextHref("databases", "dpg-a", line())!),
    ).toEqual({
      path: "/databases/dpg-a",
      search: {
        tab: "logs",
        range: "custom",
        rangeStart: "2026-11-20T10:20:15.250Z",
        rangeEnd: "2026-11-20T10:40:15.250Z",
        contextLine: "id:abc",
      },
    });
    expect(
      query(datastoreLogContextHref("keyvalue", "red-a", line())!).path,
    ).toBe("/keyvalue/red-a");
  });

  it("keeps a project view's own view, in Render's r key", () => {
    expect(
      query(
        projectLogContextHref(
          "prj-a",
          SavedSearchType.RequestLogs,
          "svs-1",
          line(),
        )!,
      ),
    ).toEqual({
      path: "/project/prj-a/observability/logs/request",
      search: {
        savedSearchId: "svs-1",
        r: "custom",
        rangeStart: "2026-11-20T10:20:15.250Z",
        rangeEnd: "2026-11-20T10:40:15.250Z",
        contextLine: "id:abc",
      },
    });
  });

  it("offers none for a line without a time, and leaves out a key too long for a link", () => {
    expect(logContextSearch(line({ timestamp: "" }))).toBeNull();
    expect(serviceLogContextHref("srv-a", line({ timestamp: "" }))).toBe(
      undefined,
    );
    const long = line({ key: `1|i|${"x".repeat(600)}` });
    expect(logContextSearch(long)?.contextLine).toBeUndefined();
  });
});

// SSH into instance (w3/m528): Render links `${servicePath}/shell/${instance}`
// for a running instance and disables the item for one that is not.
describe("serviceLineShell (w3/m528)", () => {
  const line = (instance: string) =>
    ({
      key: "id:x",
      timestamp: "2026-11-20T10:00:00Z",
      instance,
      message: "m",
      spans: null,
      resource: "",
      type: "app",
      level: "",
      method: "",
      statusCode: "",
      host: "",
      path: "",
    }) satisfies LogLine;
  const shell = serviceLineShell("srv-a", new Set(["srv-a-up"]));

  it("opens the Shell on a running instance", () => {
    expect(shell(line("srv-a-up"))).toEqual({
      href: "/services/srv-a/shell?instance=srv-a-up",
    });
  });

  it("is disabled for an instance that is not running, and absent without one", () => {
    expect(shell(line("srv-a-gone"))).toEqual({ href: null });
    expect(shell(line(""))).toBeNull();
  });
});

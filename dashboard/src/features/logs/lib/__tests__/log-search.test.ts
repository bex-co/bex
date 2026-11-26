import { describe, expect, it } from "vitest";
import {
  CLEARED_LOG_SEARCH,
  logFiltersFromSearch,
  logFiltersToSearch,
  serviceLogRange,
  logContextSearch,
  logSearchEquals,
  parseLogSearch,
  serviceLineShell,
  serviceLogContextHref,
} from "../log-search";
import { parseRangeSearch } from "@/features/metrics/lib/range";
import { datastoreLogContextHref } from "../datastore-log-search";
import { projectLogContextHref } from "../project-log-search";
import { SavedSearchType } from "@/graphql/definitions";
import { EMPTY_LOG_FILTERS, type LogFilters, type LogLine } from "../../types";

describe("log range URL state", () => {
  it("round-trips a supported selection through route validation", () => {
    const restored = parseLogSearch({ range: "24h" });
    expect(restored).toEqual({ range: "24h" });
    expect(serviceLogRange(restored).id).toBe("24h");
  });

  // Render's Logs never list Metrics' 30m and 12h (w3/100): an old link
  // naming one opens on the nearest log range.
  it("opens an old 30m or 12h link on Last hour or Last 24 hours", () => {
    expect(serviceLogRange(parseLogSearch({ range: "30m" })).id).toBe("1h");
    expect(serviceLogRange(parseLogSearch({ range: "12h" })).id).toBe("24h");
    expect(serviceLogRange(parseLogSearch({ r: "12h" })).id).toBe("24h");
    expect(serviceLogRange(parseLogSearch({ r: "15m" })).id).toBe("1h");
  });

  // Live tail is a range (w3/m537), spelled as Render spells it.
  it("round-trips Live tail as range=live, and reads Render's r=live", () => {
    const restored = parseLogSearch({ range: "live" });
    expect(restored).toEqual({ range: "live" });
    expect(serviceLogRange(restored)).toEqual({ id: "live" });
    expect(parseLogSearch({ r: "live" })).toEqual({ range: "live" });
    // A stray bound beside it is dropped.
    expect(
      parseLogSearch({ range: "live", rangeStart: "2026-07-01T00:00:00Z" }),
    ).toEqual({ range: "live" });
  });

  it("keeps Live tail out of Metrics' range keys", () => {
    expect(parseRangeSearch({ range: "live" }).range).toBeUndefined();
  });

  it("rejects malformed values and restores the one-hour default", () => {
    const restored = parseLogSearch({ range: "31d" });
    expect(restored).toStrictEqual(CLEARED_LOG_SEARCH);
    expect(serviceLogRange(restored).id).toBe("1h");
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
    const range = serviceLogRange(restored);
    expect(range.id).toBe("custom");
    expect(range).toMatchObject({
      startTime: "2026-07-01T00:00:00.000Z",
      endTime: "2026-07-01T06:00:00.000Z",
    });
  });

  it("ignores a custom range whose bounds are missing or reversed", () => {
    // reversed bounds don't validate → falls back to the default
    expect(
      serviceLogRange(
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
    expect(logFiltersToSearch(filters)).toEqual({
      type: "request",
      level: "error",
      method: "GET",
      statusCode: "5xx",
      instance: "web-abc12",
      path: "/api",
      text: "boom",
    });
  });

  it("omits defaults so an unfiltered view keeps a clean URL", () => {
    expect(logFiltersToSearch(EMPTY_LOG_FILTERS)).toEqual({});
  });

  it("serializes only the filters that are set", () => {
    expect(
      logFiltersToSearch({ ...EMPTY_LOG_FILTERS, level: "error" }),
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

  // The Live switch's `?live=0` (w3/m537): every range but Live tail is
  // paused now, so the link opens paused on the range it names, and `live`
  // is a tombstone that leaves the URL on the first write.
  it("opens a live=0 link paused on its range, with live as a tombstone", () => {
    for (const live of [0, "0", false, 1, true]) {
      expect(parseLogSearch({ live })).toStrictEqual(CLEARED_LOG_SEARCH);
    }
    expect(serviceLogRange(parseLogSearch({ live: 0 })).id).toBe("1h");
    expect(serviceLogRange(parseLogSearch({ live: 0, range: "4h" })).id).toBe(
      "4h",
    );
    expect(
      serviceLogRange(
        parseLogSearch({
          live: 0,
          range: "custom",
          rangeStart: "2026-07-01T00:00:00.000Z",
          rangeEnd: "2026-07-01T06:00:00.000Z",
        }),
      ).id,
    ).toBe("custom");
    expect("live" in CLEARED_LOG_SEARCH).toBe(true);
    expect(CLEARED_LOG_SEARCH.live).toBeUndefined();
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
    expect(serviceLogRange(search).id).toBe("1h");
  });

  it("degrades a retired preset id (pre-w5/m42 bookmark) to the default range", () => {
    for (const retired of ["3h", "6h", "1d"]) {
      const search = parseLogSearch({ range: retired });
      expect(search.range).toBeUndefined();
      expect(serviceLogRange(search).id).toBe("1h");
    }
  });

  it("never re-emits alias keys: the serializer writes canonical keys only", () => {
    const search = parseLogSearch({ t: "request", r: "6h" });
    const out = logFiltersToSearch(logFiltersFromSearch(search));
    expect(out).toEqual({ type: "request" });
    expect("t" in out).toBe(false);
    expect("r" in out).toBe(false);
  });
});

describe("logSearchEquals (the route's skip-when-unchanged guard)", () => {
  it("treats a round-tripped state as equal so the mount sync never navigates", () => {
    const search = parseLogSearch({
      range: "live",
      type: "app",
      level: "error",
    });
    const next = {
      range: search.range,
      ...logFiltersToSearch(logFiltersFromSearch(search)),
    };
    expect(logSearchEquals(search, next)).toBe(true);
  });

  it("detects a real change on any filter key", () => {
    expect(logSearchEquals({ level: "error" }, { level: "warn" })).toBe(false);
    expect(logSearchEquals({}, { range: "live" })).toBe(false);
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

  it("opens the service Logs tab unfiltered, on a custom range ten minutes either side of the line, the line marked", () => {
    const href = serviceLogContextHref("srv-a", line())!;
    const { path, search } = query(href);
    expect(path).toBe("/services/srv-a/logs");
    // A custom range never tails, so the link needs no `live=0` (w3/m537).
    expect(search).toEqual({
      range: "custom",
      rangeStart: "2026-11-20T10:20:15.250Z",
      rangeEnd: "2026-11-20T10:40:15.250Z",
      contextLine: "id:abc",
    });
    // The route reads back exactly that state, with no filter.
    const parsed = parseLogSearch(search);
    expect(logFiltersFromSearch(parsed)).toEqual(EMPTY_LOG_FILTERS);
    expect(parsed.contextLine).toBe("id:abc");
    expect(serviceLogRange(parsed).id).toBe("custom");
  });

  // Each datastore's Logs is its own page (w3/m641, w3/m649).
  it("opens a Postgres or Key Value Logs page on the line's window", () => {
    const window = {
      range: "custom",
      rangeStart: "2026-11-20T10:20:15.250Z",
      rangeEnd: "2026-11-20T10:40:15.250Z",
      contextLine: "id:abc",
    };
    expect(
      query(datastoreLogContextHref("databases", "dpg-a", line())!),
    ).toEqual({ path: "/databases/dpg-a/logs", search: window });
    expect(
      query(datastoreLogContextHref("keyvalue", "red-a", line())!),
    ).toEqual({ path: "/keyvalue/red-a/logs", search: window });
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

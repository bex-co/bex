import { describe, it, expect } from "vitest";
import {
  fromRenderLog,
  logLineKey,
  mergeLogLines,
  toLogLine,
  toLogLines,
} from "../map";
import { formatLogTimestamp } from "../format";
import type { LogLine } from "../../types";

const gqlEntry = (over: Partial<Record<string, string | null>> = {}) => ({
  __typename: "LogEntry" as const,
  timestamp: "2026-07-05T10:36:01.709Z",
  message: "hello world",
  type: "app",
  instance: "bv612",
  ...over,
});

describe("toLogLine / toLogLines", () => {
  it("maps a GraphQL LogEntry to a flat LogLine", () => {
    const line = toLogLine(gqlEntry());
    expect(line).toMatchObject({
      key: logLineKey("2026-07-05T10:36:01.709Z", "bv612", "hello world"),
      timestamp: "2026-07-05T10:36:01.709Z",
      instance: "bv612",
      message: "hello world",
      type: "app",
    });
    // `time` is the once-computed formatted clock (locale/tz-dependent shape).
    expect(line.time).toBe(formatLogTimestamp("2026-07-05T10:36:01.709Z"));
  });

  it("tolerates null fields (empty strings, default type)", () => {
    const line = toLogLine(
      gqlEntry({ timestamp: null, instance: null, type: null }),
    );
    expect(line.timestamp).toBe("");
    expect(line.instance).toBe("");
    expect(line.type).toBe("app");
  });

  it("drops null holes and undefined/null results", () => {
    expect(toLogLines(undefined)).toEqual([]);
    expect(toLogLines(null)).toEqual([]);
    expect(
      toLogLines([gqlEntry(), null, gqlEntry({ message: "b" })]),
    ).toHaveLength(2);
  });
});

describe("fromRenderLog (SSE frame)", () => {
  it("reads instance/type out of the Render labels array", () => {
    const line = fromRenderLog({
      id: "bv612-ts-abcd",
      message: "streamed line",
      timestamp: "2026-07-05T10:37:00.000Z",
      labels: [
        { name: "type", value: "app" },
        { name: "resource", value: "web" },
        { name: "instance", value: "bv612" },
      ],
    });
    expect(line.instance).toBe("bv612");
    expect(line.type).toBe("app");
    expect(line.message).toBe("streamed line");
  });

  it("produces the same key as the GraphQL mapping for the same line", () => {
    const ts = "2026-07-05T10:36:01.709Z";
    const gql = toLogLine(gqlEntry({ timestamp: ts }));
    const sse = fromRenderLog({
      message: "hello world",
      timestamp: ts,
      labels: [{ name: "instance", value: "bv612" }],
    });
    // History and live agree on the key, so a straddling line dedupes.
    expect(sse.key).toBe(gql.key);
  });

  it("defaults type to app and instance to empty when labels are missing", () => {
    const line = fromRenderLog({ message: "m", timestamp: "t" });
    expect(line.type).toBe("app");
    expect(line.instance).toBe("");
  });
});

describe("mergeLogLines", () => {
  const line = (key: string): LogLine => ({
    key,
    timestamp: key,
    instance: "i",
    message: key,
    type: "app",
  });

  it("appends live lines after history, in order", () => {
    const merged = mergeLogLines(
      [line("a"), line("b")],
      [line("c"), line("d")],
    );
    expect(merged.map((l) => l.key)).toEqual(["a", "b", "c", "d"]);
  });

  it("drops live lines whose key already appears in history", () => {
    const merged = mergeLogLines(
      [line("a"), line("b")],
      [line("b"), line("c")],
    );
    expect(merged.map((l) => l.key)).toEqual(["a", "b", "c"]);
  });

  it("dedupes within the live batch too", () => {
    const merged = mergeLogLines([], [line("x"), line("x"), line("y")]);
    expect(merged.map((l) => l.key)).toEqual(["x", "y"]);
  });
});

describe("formatLogTimestamp", () => {
  it("renders blank for empty or unparseable input", () => {
    expect(formatLogTimestamp("")).toBe("");
    expect(formatLogTimestamp("not-a-date")).toBe("");
  });

  it("renders a clock time for a valid RFC3339 stamp", () => {
    const out = formatLogTimestamp("2026-07-05T10:36:01.709Z");
    // Locale/timezone-independent shape: HH:MM:SS with an AM/PM marker.
    expect(out).toMatch(/\d{1,2}:\d{2}:\d{2}/);
  });
});

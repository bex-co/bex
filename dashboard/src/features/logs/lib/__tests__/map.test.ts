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
  level: null,
  method: null,
  statusCode: null,
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
    expect(line.level).toBe("");
    expect(line.method).toBe("");
    expect(line.statusCode).toBe("");
  });

  it("maps the request-line labels (type/method/statusCode)", () => {
    const line = toLogLine(
      gqlEntry({
        type: "request",
        method: "GET",
        statusCode: "200",
        level: null,
      }),
    );
    expect(line.type).toBe("request");
    expect(line.method).toBe("GET");
    expect(line.statusCode).toBe("200");
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
    expect(line.method).toBe("");
    expect(line.statusCode).toBe("");
  });

  it("reads request-line method/statusCode from the labels array", () => {
    const line = fromRenderLog({
      message: '{"RequestMethod":"POST"}',
      timestamp: "2026-07-05T10:37:00.000Z",
      labels: [
        { name: "type", value: "request" },
        { name: "method", value: "POST" },
        { name: "statusCode", value: "503" },
      ],
    });
    expect(line.type).toBe("request");
    expect(line.method).toBe("POST");
    expect(line.statusCode).toBe("503");
  });
});

describe("mergeLogLines", () => {
  const line = (key: string): LogLine => ({
    key,
    timestamp: key,
    time: key,
    instance: "i",
    message: key,
    type: "app",
    level: "",
    method: "",
    statusCode: "",
    spans: null,
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

  // w9/m63 t001: ANSI is parsed once at ingest (`spans`), not per render.
  it("parses ANSI into spans at ingest, and leaves a plain line null", () => {
    const esc = "\u001b";
    const colored = fromRenderLog({ message: `${esc}[32mok${esc}[0m` });
    expect(colored.spans).not.toBeNull();
    expect(colored.spans!.map((s) => s.text).join("")).toBe("ok");

    const plain = fromRenderLog({ message: "no escapes here" });
    expect(plain.spans).toBeNull();
  });
});

import { describe, it, expect } from "vitest";
import {
  LogIdentity,
  dedupeLogLines,
  fromRenderLog,
  logLineKey,
  compareLogTimestamps,
  mergeLogLines,
  toLogLine,
  toLogLines,
} from "../map";
import { formatLogTimestamp } from "../format";
import type { LogLine } from "../../types";

const gqlEntry = (over: Partial<Record<string, string | null>> = {}) => ({
  __typename: "LogEntry" as const,
  id: null as string | null,
  timestamp: "2026-07-05T10:36:01.709Z",
  message: "hello world",
  type: "app",
  instance: "bv612",
  level: null,
  method: null,
  statusCode: null,
  ...over,
});

describe("logLineKey", () => {
  // w9/053: a record redelivered with its timestamp at a coarser sub-second
  // precision (a reconnect replay whose stamp lost its nanoseconds) is the
  // same line — the viewer's clock display is second-precision, so the copies
  // render as identical duplicates when the raw strings differ.
  it("is timestamp-precision invariant within a millisecond", () => {
    const nano = logLineKey(
      "2026-08-20T20:51:50.123456789Z",
      "qa-cron-run-rs2qx",
      "hello-from-qa-cron",
    );
    const milli = logLineKey(
      "2026-08-20T20:51:50.123Z",
      "qa-cron-run-rs2qx",
      "hello-from-qa-cron",
    );
    expect(nano).toBe(milli);
  });

  it("keeps genuinely distinct records apart", () => {
    expect(logLineKey("2026-08-20T20:51:50.123Z", "p", "m")).not.toBe(
      logLineKey("2026-08-20T20:51:50.124Z", "p", "m"),
    );
    expect(logLineKey("2026-08-20T20:51:50.123Z", "p", "m")).not.toBe(
      logLineKey("2026-08-20T20:51:50.123Z", "p2", "m"),
    );
  });

  it("falls back to the raw string for an empty or unparseable timestamp", () => {
    expect(logLineKey("", "p", "m")).toBe("|p|m");
    expect(logLineKey("not-a-date", "p", "m")).toBe("not-a-date|p|m");
  });
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

  it("drops null holes and undefined results", () => {
    expect(toLogLines(undefined)).toEqual([]);
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

  // Stamp-bearing variant: key + RFC3339 timestamp + message.
  const at = (message: string, timestamp: string): LogLine => ({
    ...line(message),
    key: `${timestamp}|${message}`,
    timestamp,
  });
  const messages = (lines: LogLine[]) => lines.map((l) => l.message);

  // w4/m192: the live tail replays in-window output older than the historical
  // head (a new instance's START); it must read above newer history, not below.
  it("orders older live replay before newer history", () => {
    const history = [
      at("n=368", "2026-10-10T06:42:03.861865198Z"),
      at("n=369", "2026-10-10T06:42:05.862437817Z"),
    ];
    const live = [
      at("START", "2026-10-10T06:29:49.234399544Z"),
      at("n=1", "2026-10-10T06:29:51.236Z"),
      at("n=369", "2026-10-10T06:42:05.862437817Z"),
      at("n=370", "2026-10-10T06:42:07.864176178Z"),
    ];
    const historyCopy = [...history];
    const liveCopy = [...live];
    expect(messages(mergeLogLines(history, live))).toEqual([
      "START",
      "n=1",
      "n=368",
      "n=369",
      "n=370",
    ]);
    // Neither source array is mutated.
    expect(history).toEqual(historyCopy);
    expect(live).toEqual(liveCopy);
  });

  it("orders by real time, not by string, across precisions and zones", () => {
    const merged = mergeLogLines(
      [
        at("half", "2026-10-10T06:00:00.5Z"),
        at("whole", "2026-10-10T06:00:01Z"),
      ],
      [
        at("49", "2026-10-10T06:00:00.49Z"),
        at("offset", "2026-10-10T08:00:00.7+02:00"),
      ],
    );
    expect(messages(merged)).toEqual(["49", "half", "offset", "whole"]);
  });

  it("keeps sub-millisecond order inside one millisecond", () => {
    const merged = mergeLogLines(
      [at("late", "2026-10-10T06:00:00.123900Z")],
      [
        at("early", "2026-10-10T06:00:00.1230001Z"),
        at("mid", "2026-10-10T06:00:00.12345Z"),
      ],
    );
    expect(messages(merged)).toEqual(["early", "mid", "late"]);
  });

  it("keeps source order for equal stamps and pins stampless lines to their neighbor", () => {
    const ts = "2026-10-10T06:00:01Z";
    const merged = mergeLogLines(
      [at("h1", ts), at("h-stampless", ""), at("h2", "2026-10-10T06:00:03Z")],
      [at("l1", ts), at("l0", "2026-10-10T06:00:00Z")],
    );
    expect(messages(merged)).toEqual(["l0", "h1", "h-stampless", "l1", "h2"]);
  });

  it("drops stamped lines before the window start but keeps stampless ones", () => {
    const merged = mergeLogLines(
      [at("old", "2026-10-10T05:00:00Z"), at("stampless", "")],
      [at("in", "2026-10-10T06:00:00Z")],
      Date.parse("2026-10-10T05:30:00Z"),
    );
    expect(messages(merged)).toEqual(["stampless", "in"]);
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

  // w4/m96: the same cron emission arrives on both paths — the historical
  // GraphQL row and the live SSE frame. Once the backend canonicalizes the
  // historical record's transport newline (internal/logs/loki.go), both carry
  // identical message bytes, so the straddling line renders once, not twice.
  it("dedupes one cron emission across the historical and live wire shapes", () => {
    const ts = "2026-09-07T05:23:04.824789480Z";
    const instance = "tea-x-qa-20260906-cron-29812643-lwzhj";
    const history = toLogLine(
      gqlEntry({ timestamp: ts, instance, message: "qa-cron-success" }),
    );
    const live = fromRenderLog({
      message: "qa-cron-success",
      timestamp: ts,
      labels: [
        { name: "type", value: "app" },
        { name: "instance", value: instance },
      ],
    });
    const merged = mergeLogLines([history], [live]);
    expect(merged).toHaveLength(1);
    expect(merged[0].message).toBe("qa-cron-success");
  });

  // Regression guard: the frontend key is byte-exact by design, so a historical
  // record that STILL carried its transport newline (the pre-fix bug) keys
  // differently and double-renders. This is why the fix belongs at the backend
  // read boundary, not a frontend trim that would also erase real data newlines.
  it("would double-render if the historical message kept its transport newline", () => {
    const ts = "2026-09-07T05:23:04.824789480Z";
    const instance = "qa-cron-29812643-lwzhj";
    const historyWithNewline = toLogLine(
      gqlEntry({ timestamp: ts, instance, message: "qa-cron-success\n" }),
    );
    const live = fromRenderLog({
      message: "qa-cron-success",
      timestamp: ts,
      labels: [{ name: "instance", value: instance }],
    });
    expect(historyWithNewline.key).not.toBe(live.key);
    expect(mergeLogLines([historyWithNewline], [live])).toHaveLength(2);
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

// w4/m217 (a751): twenty records logged inside one millisecond, same pod and
// message, distinct nanosecond stamps and backend ids. The reader kept one,
// because its key was the millisecond bucket. Distinct ids now stay distinct,
// while the w9/053 guarantee holds: one record redelivered at millisecond
// precision is still one row, whichever copy arrives first.
describe("record identity within one millisecond (w4/m217)", () => {
  const ms = "2026-10-16T12:24:33.681";
  const fine = (n: number) => {
    const timestamp = `${ms}${String(100000 + n).slice(1)}Z`; // .681xxxxxZ
    return toLogLine(
      gqlEntry({
        id: `pod-a-${timestamp}-c0ffee`,
        timestamp,
        message: "QA_A751 same",
        instance: "pod-a",
      }),
    );
  };
  const coarse = (id?: string) =>
    fromRenderLog({
      id,
      timestamp: `${ms}Z`,
      message: "QA_A751 same",
      labels: [{ name: "instance", value: "pod-a" }],
    });
  const twenty = Array.from({ length: 20 }, (_, n) => fine(n + 1));

  it("keeps twenty distinct records through dedupe and the paused-history merge", () => {
    expect(new Set(twenty.map((line) => line.key)).size).toBe(20);
    expect(dedupeLogLines(twenty)).toHaveLength(20);
    expect(mergeLogLines(twenty, [])).toHaveLength(20);
  });

  // The w9/053 fallback: a replay that lost its nanoseconds and carries no
  // id has only the millisecond bucket to go on, so it pairs with the one
  // identified record there.
  it("draws one row for a record and its ID-less millisecond replay, either order", () => {
    const record = fine(7);
    const replay = coarse();
    expect(dedupeLogLines([record, replay])).toEqual([record]);
    expect(dedupeLogLines([replay, record])).toEqual([replay]);
    expect(mergeLogLines([record], [replay, record])).toEqual([record]);
  });

  it("draws one row for one id redelivered with a coarser displayed stamp", () => {
    const record = fine(7);
    const redelivered = coarse(`pod-a-${record.timestamp}-c0ffee`);
    expect(redelivered.key).toBe(record.key);
    expect(dedupeLogLines([record, redelivered])).toEqual([record]);
    expect(dedupeLogLines([redelivered, record])).toEqual([redelivered]);
    expect(mergeLogLines([record], [redelivered])).toEqual([record]);
  });

  // a872: a different record stamped exactly on the millisecond (Go's
  // RFC3339Nano drops trailing zeros) carries its own id. Two known ids are
  // two records, even when one stamp is shorter.
  it("keeps an exact-millisecond record distinct from a finer one (a872)", () => {
    const entry = (timestamp: string) =>
      toLogLine(
        gqlEntry({
          id: `pod-a-${timestamp}-1a2b3c4d`,
          timestamp,
          instance: "pod-a",
          message: "QA_A872 FAST",
        }),
      );
    const fineRecord = entry("2026-10-17T09:10:19.863264187Z");
    const exact = entry("2026-10-17T09:10:19.863Z");
    expect(dedupeLogLines([fineRecord, exact])).toEqual([fineRecord, exact]);
    expect(dedupeLogLines([exact, fineRecord])).toEqual([exact, fineRecord]);
    expect(mergeLogLines([fineRecord], [exact])).toHaveLength(2);
    expect(mergeLogLines([exact], [fineRecord])).toHaveLength(2);
  });

  it("keeps the other records when one coarse replay joins them", () => {
    expect(dedupeLogLines([...twenty, coarse(), coarse()])).toHaveLength(20);
    expect(dedupeLogLines([coarse(), ...twenty])).toHaveLength(20);
  });

  it("keeps the legacy bucket key for a line without an id", () => {
    const legacy = toLogLines([
      {
        __typename: "LogEntry",
        id: null,
        timestamp: `${ms}100Z`,
        message: "x",
        type: "app",
        instance: "p",
        level: null,
        method: null,
        statusCode: null,
      },
      {
        __typename: "LogEntry",
        id: null,
        timestamp: `${ms}200Z`,
        message: "x",
        type: "app",
        instance: "p",
        level: null,
        method: null,
        statusCode: null,
      },
    ]);
    expect(legacy[0].key).toBe(logLineKey(`${ms}100Z`, "p", "x"));
    expect(dedupeLogLines(legacy)).toHaveLength(1);
  });

  it("forgets an evicted record once, keeping its bucket neighbor paired", () => {
    const identity = new LogIdentity();
    const evicted = fine(1);
    const shown = fine(2);
    expect(identity.admit(evicted)).toBe(true);
    expect(identity.admit(shown)).toBe(true);
    identity.forget(evicted);
    identity.forget(evicted); // StrictMode's double-invoked updater
    // The ID-less replay still pairs with the record on screen.
    expect(identity.admit(coarse())).toBe(false);
  });

  it("admits an evicted record again", () => {
    const identity = new LogIdentity();
    const record = fine(3);
    expect(identity.admit(record)).toBe(true);
    expect(identity.admit(fine(3))).toBe(false);
    identity.forget(record);
    expect(identity.admit(fine(3))).toBe(true);
  });
});

describe("compareLogTimestamps (w8/m116)", () => {
  const sorted = (stamps: string[]) => [...stamps].sort(compareLogTimestamps);

  it("orders inside one millisecond by the real nanoseconds", () => {
    expect(
      sorted(["2026-10-18T06:02:30.455789Z", "2026-10-18T06:02:30.455123Z"]),
    ).toEqual(["2026-10-18T06:02:30.455123Z", "2026-10-18T06:02:30.455789Z"]);
  });

  it("orders trailing-zero-trimmed stamps by value, not by string", () => {
    // Go's RFC3339Nano trims zeros: .1Z is 100ms, after .09Z and before .12Z.
    expect(
      sorted([
        "2026-10-18T06:02:30.12Z",
        "2026-10-18T06:02:30.1Z",
        "2026-10-18T06:02:30.09Z",
      ]),
    ).toEqual([
      "2026-10-18T06:02:30.09Z",
      "2026-10-18T06:02:30.1Z",
      "2026-10-18T06:02:30.12Z",
    ]);
  });

  it("sorts unparseable stamps first, consistently", () => {
    expect(
      sorted(["2026-10-18T06:02:33Z", "", "2026-10-18T06:02:31Z", "nope"]),
    ).toEqual(["", "nope", "2026-10-18T06:02:31Z", "2026-10-18T06:02:33Z"]);
  });
});

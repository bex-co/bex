import { describe, expect, it } from "vitest";
import { parseAnsi } from "../ansi";
import {
  CLAMP_CHARS,
  CLAMP_LINES,
  clampMessage,
  logLineText,
  logListRows,
  logRowsText,
  shortInstance,
} from "../log-rows";
import type { LogLine } from "../../types";

// Tests run in UTC (vitest.config.ts pins TZ), so a local day is a UTC day.
function line(key: string, timestamp: string, over: Partial<LogLine> = {}) {
  return {
    key,
    timestamp,
    instance: "",
    message: key,
    spans: null,
    resource: "",
    type: "app",
    level: "",
    method: "",
    statusCode: "",
    host: "",
    path: "",
    ...over,
  } satisfies LogLine;
}

const shape = (rows: ReturnType<typeof logListRows>) =>
  rows.map((row) =>
    row.kind === "day" ? `day ${new Date(row.day).toISOString()}` : row.key,
  );

describe("logListRows (w3/m509)", () => {
  const acrossMidnight = [
    line("a", "2026-11-18T23:59:58Z"),
    line("b", "2026-11-18T23:59:59Z"),
    line("c", "2026-11-19T00:00:01Z"),
  ];

  it("adds no separator to one day's lines", () => {
    expect(shape(logListRows(acrossMidnight.slice(0, 2), "asc"))).toEqual([
      "a",
      "b",
    ]);
  });

  it("marks each day of a range across midnight, the first one included", () => {
    expect(shape(logListRows(acrossMidnight, "asc"))).toEqual([
      "day 2026-11-18T00:00:00.000Z",
      "a",
      "b",
      "day 2026-11-19T00:00:00.000Z",
      "c",
    ]);
  });

  it("leads each day with its separator in descending order too", () => {
    expect(shape(logListRows(acrossMidnight, "desc"))).toEqual([
      "day 2026-11-19T00:00:00.000Z",
      "c",
      "day 2026-11-18T00:00:00.000Z",
      "b",
      "a",
    ]);
  });

  it("keeps a line without a time in the day before it", () => {
    const rows = logListRows(
      [acrossMidnight[0], line("x", ""), acrossMidnight[2]],
      "asc",
    );
    expect(shape(rows)).toEqual([
      "day 2026-11-18T00:00:00.000Z",
      "a",
      "x",
      "day 2026-11-19T00:00:00.000Z",
      "c",
    ]);
  });

  it("keys a separator by its day, so it survives pages arriving around it", () => {
    const before = logListRows(acrossMidnight, "asc");
    const after = logListRows(
      [line("older", "2026-11-18T12:00:00Z"), ...acrossMidnight],
      "asc",
    );
    const dayKeys = (rows: typeof before) =>
      rows.filter((row) => row.kind === "day").map((row) => row.key);
    expect(dayKeys(after)).toEqual(dayKeys(before));
    expect(new Set(after.map((row) => row.key)).size).toBe(after.length);
  });
});

describe("clampMessage (w3/m509)", () => {
  it("leaves a message within both limits whole", () => {
    expect(clampMessage({ message: "short", spans: null })).toBeNull();
    const lines = Array.from({ length: CLAMP_LINES }, (_, i) => `l${i}`);
    expect(clampMessage({ message: lines.join("\n"), spans: null })).toBeNull();
  });

  it("cuts a message after its first lines", () => {
    const lines = Array.from({ length: CLAMP_LINES + 5 }, (_, i) => `l${i}`);
    const clamped = clampMessage({ message: lines.join("\n"), spans: null });
    expect(clamped?.text).toBe(lines.slice(0, CLAMP_LINES).join("\n"));
  });

  it("cuts a long single line at the character limit", () => {
    const clamped = clampMessage({
      message: "x".repeat(CLAMP_CHARS + 1),
      spans: null,
    });
    expect(clamped?.text).toHaveLength(CLAMP_CHARS);
  });

  it("cuts a styled message's spans at the same point, keeping their styles", () => {
    const message = `\u001b[31m${"r".repeat(CLAMP_CHARS - 2)}\u001b[0mtail-that-goes-on`;
    const clamped = clampMessage({ message, spans: parseAnsi(message) });
    expect(clamped?.spans?.map((span) => span.text).join("")).toBe(
      clamped?.text,
    );
    expect(clamped?.text).toHaveLength(CLAMP_CHARS);
    expect(clamped?.spans?.[0].className).toContain("red");
  });
});

describe("shortInstance", () => {
  it("shows a public instance id's first five suffix characters", () => {
    expect(shortInstance("srv-abc-0123456789abcdefghij")).toBe("01234");
    expect(shortInstance("web-7d9f-bv612")).toBe("bv612");
    expect(shortInstance("pod")).toBe("pod");
  });
});

describe("logRowsText (w3/m509)", () => {
  const format = {
    time: (timestamp: string) => timestamp.slice(11, 19),
    day: (day: number) => new Date(day).toISOString().slice(0, 10),
  };

  it("writes each row as it reads, in the order shown", () => {
    const rows = logListRows(
      [
        line("a", "2026-11-18T23:59:58Z", {
          instance: "srv-abc-0123456789abcdefghij",
          resource: "srv-abc",
          message: "\u001b[32mready\u001b[0m",
          spans: parseAnsi("\u001b[32mready\u001b[0m"),
        }),
        line("b", "2026-11-19T00:00:01Z", {
          type: "request",
          method: "GET",
          statusCode: "404",
          host: "example.com",
          path: "/missing",
          message: 'clientIP="198.51.100.3"',
        }),
      ],
      "desc",
    );
    expect(logRowsText(rows, { ...format, resource: () => "storefront" })).toBe(
      [
        "2026-11-19",
        '00:00:01 GET 404 example.com/missing clientIP="198.51.100.3"',
        "2026-11-18",
        "23:59:58 [storefront] [01234] ready",
      ].join("\n"),
    );
  });

  it("leaves out the time when the list hides it, and the resource when it names none", () => {
    const rows = logListRows(
      [line("a", "2026-11-18T10:00:00Z", { resource: "srv-abc" })],
      "asc",
    );
    expect(logRowsText(rows, { ...format, time: null })).toBe("a");
  });

  it("copies a clamped message whole", () => {
    const long = "y".repeat(CLAMP_CHARS * 2);
    const rows = logListRows(
      [line("a", "2026-11-18T10:00:00Z", { message: long })],
      "asc",
    );
    expect(logRowsText(rows, { ...format, time: null })).toBe(long);
  });
});

// Render's row menu copies one line with `En` (w3/m528): `[label] METHOD
// STATUS host+path`, then the text with ANSI stripped; no time, no instance.
describe("logLineText (w3/m528)", () => {
  it("copies an app line's text, its styling stripped, without its time or instance", () => {
    expect(
      logLineText(
        line("a", "2026-11-18T10:00:00Z", {
          instance: "srv-abc-0123456789abcdefghij",
          message: "\u001b[32mready\u001b[0m",
          spans: parseAnsi("\u001b[32mready\u001b[0m"),
        }),
      ),
    ).toBe("ready");
  });

  it("leads a request line with its method, status, host and path", () => {
    expect(
      logLineText(
        line("b", "2026-11-19T00:00:01Z", {
          type: "request",
          method: "GET",
          statusCode: "404",
          host: "example.com",
          path: "/missing",
          message: 'clientIP="198.51.100.3"',
        }),
      ),
    ).toBe('GET 404 example.com/missing clientIP="198.51.100.3"');
  });

  it("names a project view line's resource first, and copies a clamped message whole", () => {
    const long = "y".repeat(CLAMP_CHARS * 2);
    expect(
      logLineText(
        line("c", "2026-11-18T10:00:00Z", {
          resource: "srv-abc",
          message: long,
        }),
        "storefront",
      ),
    ).toBe(`[storefront] ${long}`);
  });

  it("is blank for a blank line, which the menu cannot copy", () => {
    expect(
      logLineText(line("d", "2026-11-18T10:00:00Z", { message: "" })).trim(),
    ).toBe("");
  });
});

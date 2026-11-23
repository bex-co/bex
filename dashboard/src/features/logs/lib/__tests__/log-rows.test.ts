import { describe, expect, it } from "vitest";
import { parseAnsi } from "../ansi";
import {
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

describe("shortInstance", () => {
  it("shows a public instance id's first five suffix characters", () => {
    expect(shortInstance("srv-abc-0123456789abcdefghij")).toBe("01234");
    expect(shortInstance("web-7d9f-bv612")).toBe("bv612");
    expect(shortInstance("pod")).toBe("pod");
  });
});

// Render's Copy logs writes each record with `Dn` (w3/090, its dashboard
// bundle read by w3/m528): `${timestamp} ${En(line)}`, the record's own
// timestamp, then `[resource] METHOD STATUS host+path text`; no date rows and
// no instance.
describe("logRowsText (w3/090)", () => {
  const app = line("a", "2026-11-18T23:59:58.123456789Z", {
    instance: "srv-abc-0123456789abcdefghij",
    resource: "srv-abc",
    message: "\u001b[32mready\u001b[0m",
    spans: parseAnsi("\u001b[32mready\u001b[0m"),
  });
  const request = line("b", "2026-11-19T00:00:01Z", {
    type: "request",
    instance: "srv-abc-0123456789abcdefghij",
    resource: "srv-def",
    method: "GET",
    statusCode: "404",
    host: "example.com",
    path: "/missing",
    message: 'clientIP="198.51.100.3"',
  });

  it("writes an app line as its timestamp and text, its styling stripped, with no instance", () => {
    expect(logRowsText(logListRows([app], "asc"))).toBe(
      "2026-11-18T23:59:58.123456789Z ready",
    );
  });

  it("writes a request line's method, status, host and path after its timestamp", () => {
    expect(logRowsText(logListRows([request], "asc"))).toBe(
      '2026-11-19T00:00:01Z GET 404 example.com/missing clientIP="198.51.100.3"',
    );
  });

  it("names a project view line's resource, by id when the view does not name it", () => {
    const names = new Map([["srv-abc", "storefront"]]);
    expect(
      logRowsText(logListRows([app, request], "asc"), (id) => names.get(id)),
    ).toBe(
      [
        "2026-11-18T23:59:58.123456789Z [storefront] ready",
        '2026-11-19T00:00:01Z [srv-def] GET 404 example.com/missing clientIP="198.51.100.3"',
      ].join("\n"),
    );
  });

  it("writes one line per record in the order shown, with no date rows", () => {
    const rows = logListRows([app, request], "desc");
    // The list shows a date row over each day; the copy has none.
    expect(rows.filter((row) => row.kind === "day")).toHaveLength(2);
    expect(logRowsText(rows).split("\n")).toEqual([
      '2026-11-19T00:00:01Z GET 404 example.com/missing clientIP="198.51.100.3"',
      "2026-11-18T23:59:58.123456789Z ready",
    ]);
  });

  it("copies a long message whole, and a line without a time as its text", () => {
    const long = "y".repeat(2000);
    expect(
      logRowsText(
        logListRows(
          [line("a", "2026-11-18T10:00:00Z", { message: long }), line("x", "")],
          "asc",
        ),
      ),
    ).toBe(`2026-11-18T10:00:00Z ${long}\nx`);
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

  it("copies Render's coloured `==>` lines as their words, as Render's copy reads (w32/m576)", () => {
    for (const [message, words] of [
      [
        "\u001b[0;34m\u001b[1m==> \u001b[0m\u001b[1mDeploying...\u001b[0m",
        "==> Deploying...",
      ],
      [
        "\u001b[32m\u001b[1m==>\u001b(B\u001b[m \u001b[1mRunning 'node main.js'\u001b(B\u001b[m",
        "==> Running 'node main.js'",
      ],
      [
        "\u001b[32;1m==>\u001b[0;22m \u001b[1mBuild successful 🎉\u001b[22m",
        "==> Build successful 🎉",
      ],
    ]) {
      expect(
        logLineText(
          line("r", "2026-11-22T10:00:00Z", {
            type: "build",
            message,
            spans: parseAnsi(message),
          }),
        ),
      ).toBe(words);
    }
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

  it("names a project view line's resource first, and copies a long message whole", () => {
    const long = "y".repeat(2000);
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

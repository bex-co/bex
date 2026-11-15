import { describe, expect, it } from "vitest";

import {
  formatClockTime,
  formatDateLong,
  formatDateTime,
  formatInstantDetails,
  formatMediumDateTime,
  formatShortDateTime,
  formatTimeOfDay,
  isValidTimeZone,
} from "@/common/lib/format";

// vitest runs with TZ=UTC. Each absolute time reads in the dashboard language,
// never the browser's; en stays exactly what it was before zh existed (w4/600).
const INSTANT = "2026-11-12T07:06:22Z";

describe("absolute times follow the dashboard language", () => {
  it("formatDateTime", () => {
    expect(formatDateTime(INSTANT, "en")).toBe("November 12, 2026 at 7:06 AM");
    expect(formatDateTime(INSTANT)).toBe("November 12, 2026 at 7:06 AM");
    expect(formatDateTime(INSTANT, "zh")).toBe("2026年11月12日 上午7:06");
  });

  it("formatDateLong", () => {
    expect(formatDateLong(INSTANT, "en")).toBe("November 12, 2026");
    expect(formatDateLong(INSTANT, "zh-CN")).toBe("2026年11月12日");
  });

  it("formatInstantDetails", () => {
    expect(formatInstantDetails(INSTANT, "en")?.utc).toBe(
      "November 12, 2026 at 7:06:22 AM UTC",
    );
    const zh = formatInstantDetails(INSTANT, "zh");
    expect(zh?.utc).toMatch(/^2026年11月12日 .*上午7:06:22/);
    expect(zh?.unix).toBe("1794467182");
  });

  it("formatClockTime, the log line clock", () => {
    expect(formatClockTime(INSTANT, "en")).toBe("07:06:22 AM");
    expect(formatClockTime(INSTANT, "zh")).toBe("上午07:06:22");
    expect(formatClockTime("", "zh")).toBe("");
    expect(formatClockTime("not-a-date")).toBe("");
  });

  // en keeps what toLocaleString("en-US") gave these call sites; zh reads the
  // 12-hour clock its own Intl default would render 24-hour.
  it("formatTimeOfDay, a chart point", () => {
    expect(formatTimeOfDay(Date.parse(INSTANT), "en")).toBe("7:06:22 AM");
    expect(formatTimeOfDay(Date.parse(INSTANT), "zh")).toBe("上午7:06:22");
  });

  it("formatShortDateTime and formatMediumDateTime", () => {
    expect(formatShortDateTime(INSTANT, "en")).toBe("11/12/2026, 7:06:22 AM");
    expect(formatShortDateTime(INSTANT, "zh")).toBe("2026/11/12 上午7:06:22");
    expect(formatMediumDateTime(INSTANT, "en")).toBe("Nov 12, 2026, 7:06 AM");
    expect(formatMediumDateTime(INSTANT, "zh")).toBe("2026/11/12 上午07:06");
  });

  // ko/ja/es/pt/ru/de read their own CLDR date and clock conventions, never
  // en's forced 12-hour AM/PM (fragments, not whole strings: CLDR wording
  // drifts between Node's ICU versions).
  it.each([
    ["ja", "2026年11月12日", /7:06/],
    ["ko", "2026년 11월 12일", /오전 7:06/],
    ["es", "12 de noviembre de 2026", /7:06/],
    ["pt", "12 de novembro de 2026", /07:06/],
    ["ru", "12 ноября 2026", /07:06/],
    ["de", "12. November 2026", /07:06/],
  ])("formatDateTime and formatDateLong in %s", (language, date, time) => {
    const dateTime = formatDateTime(INSTANT, language) ?? "";
    expect(dateTime).toContain(date);
    expect(dateTime).toMatch(time);
    expect(dateTime).not.toMatch(/AM|PM/);
    expect(formatDateLong(INSTANT, language)).toContain(date);
  });

  it.each(["ja", "es", "pt", "ru", "de"])(
    "a 24-hour clock in %s",
    (language) => {
      expect(formatClockTime(INSTANT, language)).toBe("07:06:22");
      const utc = formatInstantDetails(INSTANT, language)?.utc;
      expect(utc).toMatch(/\b0?7:06:22 UTC/);
      expect(utc).not.toMatch(/AM|PM/);
    },
  );

  it("ko keeps its own 오전/오후 clock, and a region tag reads its base language", () => {
    expect(formatClockTime(INSTANT, "ko")).toBe("오전 07:06:22");
    expect(formatDateLong(INSTANT, "pt-BR")).toContain("12 de novembro");
    expect(formatDateLong(INSTANT, "fr")).toBe("November 12, 2026");
  });

  it("isValidTimeZone", () => {
    expect(isValidTimeZone("America/Los_Angeles")).toBe(true);
    expect(isValidTimeZone("Mars/Olympus")).toBe(false);
  });
});

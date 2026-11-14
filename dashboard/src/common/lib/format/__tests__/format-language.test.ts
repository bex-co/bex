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

  it("isValidTimeZone", () => {
    expect(isValidTimeZone("America/Los_Angeles")).toBe(true);
    expect(isValidTimeZone("Mars/Olympus")).toBe(false);
  });
});

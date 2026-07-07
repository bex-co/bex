import { describe, it, expect } from "vitest";
import { formatMetricValue, formatMetricShort, METRIC_LABELS } from "../format";

describe("formatMetricValue", () => {
  it("formats cpu as cores to 3 decimal places", () => {
    expect(formatMetricValue("cpu", 0.033)).toBe("0.033 cores");
    expect(formatMetricValue("cpu", 0)).toBe("0.000 cores");
  });

  it("formats bytes, choosing a unit by magnitude", () => {
    expect(formatMetricValue("bytes", 512)).toBe("512 B");
    expect(formatMetricValue("bytes", 1024)).toBe("1 KiB");
    expect(formatMetricValue("bytes", 415 * 1024 * 1024)).toBe("415 MiB");
    expect(formatMetricValue("bytes", 2 * 1024 * 1024 * 1024)).toBe("2 GiB");
  });

  it("formats seconds as whole milliseconds", () => {
    expect(formatMetricValue("seconds", 0.095)).toBe("95 ms");
    expect(formatMetricValue("seconds", 1.5804)).toBe("1580 ms");
  });

  it("formats percentage to one decimal place", () => {
    expect(formatMetricValue("percentage", 42.567)).toBe("42.6%");
  });

  it("formats count, collapsing to K above 1000", () => {
    expect(formatMetricValue("count", 1)).toBe("1");
    expect(formatMetricValue("count", 1.1)).toBe("1.1");
    expect(formatMetricValue("count", 2500)).toBe("2.5K");
  });

  it("falls back to a bare string for an unknown unit", () => {
    expect(formatMetricValue("frobs", 7)).toBe("7");
  });
});

describe("formatMetricShort", () => {
  it("is more compact than formatMetricValue for the same unit", () => {
    // Axis/tooltip labels drop the unit suffix noise formatMetricValue keeps.
    expect(formatMetricShort("cpu", 0.033)).toBe("0.03");
    expect(formatMetricShort("bytes", 415 * 1024 * 1024).length).toBeLessThan(
      formatMetricValue("bytes", 415 * 1024 * 1024).length + 4,
    );
    expect(formatMetricShort("seconds", 0.095)).toBe("95ms");
    expect(formatMetricShort("percentage", 42.567)).toBe("43%");
  });
});

describe("METRIC_LABELS", () => {
  it("covers every metric id bex-api exposes", () => {
    expect(Object.keys(METRIC_LABELS).sort()).toEqual(
      [
        "bandwidth",
        "cpu",
        "cpu_limit",
        "http_latency",
        "http_requests",
        "instance_count",
        "memory",
        "memory_limit",
      ].sort(),
    );
  });
});

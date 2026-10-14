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

// w4/296: a lightly used service's captured samples (CPU Total in cores,
// CPU/Memory Percentage) must never print as an exact zero.
describe("small positive CPU/percentage values", () => {
  it("keeps three significant digits where the fixed precision rounds to zero", () => {
    expect(formatMetricShort("cpu", 0.0002505198684260272)).toBe("0.000251");
    expect(formatMetricShort("cpu", 0.0001252599342130136)).toBe("0.000125");
    expect(formatMetricShort("percentage", 0.070953369140625)).toBe("0.0710%");
    expect(formatMetricShort("percentage", 0.0354766845703125)).toBe("0.0355%");
    expect(formatMetricValue("cpu", 0.0002505198684260272)).toBe(
      "0.000251 cores",
    );
    expect(formatMetricValue("percentage", 0.0354766845703125)).toBe("0.0355%");
  });

  it("yields distinct min/mid/max ticks for the captured domains", () => {
    for (const [unit, max] of [
      ["cpu", 0.0002505198684260272],
      ["percentage", 0.2505198684260272],
      ["percentage", 0.070953369140625],
    ] as const) {
      const ticks = [0, max / 2, max].map((v) => formatMetricShort(unit, v));
      expect(new Set(ticks).size).toBe(3);
    }
  });

  it("bounds unusually tiny values instead of padding zeros", () => {
    expect(formatMetricShort("cpu", 1.234e-9)).toBe("1.23e-9");
    expect(formatMetricShort("percentage", -0.0004)).toBe("-0.000400%");
  });

  it("keeps ordinary magnitudes and true zero on the fixed convention", () => {
    expect(formatMetricShort("cpu", 0)).toBe("0.00");
    expect(formatMetricShort("cpu", 0.005)).toBe("0.01");
    expect(formatMetricShort("percentage", 0)).toBe("0%");
    expect(formatMetricShort("percentage", 0.5)).toBe("1%");
    expect(formatMetricValue("percentage", 0)).toBe("0.0%");
    expect(formatMetricValue("percentage", 0.1)).toBe("0.1%");
    expect(formatMetricValue("cpu", 0.0005)).toBe("0.001 cores");
  });

  it("leaves bytes, seconds, count, and unknown units unchanged", () => {
    expect(formatMetricShort("bytes", 0.4)).toBe("0 B");
    expect(formatMetricShort("seconds", 0.0001)).toBe("0ms");
    expect(formatMetricShort("count", 0.01)).toBe("0.0");
    expect(formatMetricValue("seconds", 0.0001)).toBe("0 ms");
  });
});

describe("METRIC_LABELS", () => {
  it("covers every metric id bex-api exposes", () => {
    expect(Object.keys(METRIC_LABELS).sort()).toEqual(
      [
        "bandwidth",
        "cpu",
        "cpu_limit",
        "cpu_target",
        "http_latency",
        "http_requests",
        "instance_count",
        "memory",
        "memory_limit",
        "memory_target",
      ].sort(),
    );
  });
});

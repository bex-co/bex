import { describe, expect, it } from "vitest";
import {
  formatPreciseUsd,
  parseSpendLimit,
  stoppedAtSessionSpendLimit,
} from "@/features/agent-sessions/lib/spend-limit";

describe("parseSpendLimit", () => {
  it("treats blank as no ceiling", () => {
    expect(parseSpendLimit("  ")).toEqual({ ok: true, value: null });
  });
  it.each([
    ["5", 5],
    ["0.25", 0.25],
    [".5", 0.5],
    ["100000", 100000],
  ])("accepts %s", (text, value) => {
    expect(parseSpendLimit(text)).toEqual({ ok: true, value });
  });
  it.each(["0", "-1", "100000.01", "abc", "1e3", "1,5", "NaN"])(
    "rejects %s",
    (text) => {
      expect(parseSpendLimit(text)).toEqual({ ok: false });
    },
  );
});

describe("formatPreciseUsd", () => {
  it("keeps sub-cent digits but never fewer than two", () => {
    expect(formatPreciseUsd("0.001234", "en")).toBe("$0.001234");
    expect(formatPreciseUsd("5", "en")).toBe("$5.00");
  });
});

describe("stoppedAtSessionSpendLimit", () => {
  const base = {
    failureReasonCode: "AGENT_MODEL_SPEND_CAP_REACHED",
    maxSpendUSD: "5",
    outcome: { cost: { modelUSD: "5.000100" } },
  };
  it("is true once recorded spend reaches the ceiling", () => {
    expect(stoppedAtSessionSpendLimit(base)).toBe(true);
  });
  it("is false under the ceiling, without one, or for another code", () => {
    expect(
      stoppedAtSessionSpendLimit({
        ...base,
        outcome: { cost: { modelUSD: "1" } },
      }),
    ).toBe(false);
    expect(stoppedAtSessionSpendLimit({ ...base, maxSpendUSD: null })).toBe(
      false,
    );
    expect(
      stoppedAtSessionSpendLimit({ ...base, failureReasonCode: "OTHER" }),
    ).toBe(false);
  });
});

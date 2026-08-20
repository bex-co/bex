import { describe, expect, it } from "vitest";
import { collapseDoubledParts } from "@/features/agent-sessions/lib/collapse-doubled-parts";
import {
  elapsedMsFromSource,
  formatApproxDuration,
  formatElapsedDuration,
  formatStreamDuration,
  MAX_ELAPSED_MS,
} from "@/features/agent-sessions/lib/stream-duration";
import { sourceTimestampsMs } from "@/features/agent-sessions/lib/acp-parts";

const T0 = Date.parse("2026-08-19T00:00:00.000Z");
const T40 = Date.parse("2026-08-19T00:00:40.000Z");

describe("elapsedMsFromSource", () => {
  it("returns the closed interval for live and replay of the same times", () => {
    expect(elapsedMsFromSource([T0, T40], true, T40)).toBe(40_000);
    expect(elapsedMsFromSource([T0, T40], false, T40 + 5_000)).toBe(40_000);
  });

  it("grows from a single start time until settled", () => {
    expect(elapsedMsFromSource([T0], false, T0 + 12_000)).toBe(12_000);
    expect(elapsedMsFromSource([T0, T0], false, T0 + 12_000)).toBe(12_000);
    expect(elapsedMsFromSource([T0], true, T0 + 12_000)).toBe(0);
  });

  it("never goes negative on out-of-order or equal times", () => {
    expect(elapsedMsFromSource([T40, T0], true, T40)).toBe(40_000);
    expect(elapsedMsFromSource([T0, T0], true, T0)).toBe(0);
  });

  it("ignores invalid values and clamps pathological spans", () => {
    expect(elapsedMsFromSource([Number.NaN, Number.POSITIVE_INFINITY], true, T0)).toBeUndefined();
    expect(elapsedMsFromSource([], true, T0)).toBeUndefined();
    expect(
      elapsedMsFromSource([T0, T0 + MAX_ELAPSED_MS * 4], true, T0),
    ).toBe(MAX_ELAPSED_MS);
  });
});

describe("formatStreamDuration", () => {
  it("omits the tilde for persisted times and keeps it for arrival fallback", () => {
    expect(formatElapsedDuration(40_000)).toBe("40s");
    expect(formatElapsedDuration(184_000)).toBe("3m 4s");
    expect(formatApproxDuration(12_000)).toBe("~12s");
    expect(formatStreamDuration({ ms: 40_000, approximate: false })).toBe("40s");
    expect(formatStreamDuration({ ms: 12_000, approximate: true })).toBe("~12s");
  });
});

describe("reconnect dedupe", () => {
  it("does not double-count source times after a doubled replay", () => {
    const part = {
      type: "dynamic-tool" as const,
      toolName: "search",
      text: "",
      callProviderMetadata: { bex: { at: "2026-08-19T00:00:00.000Z" } },
      resultProviderMetadata: { bex: { at: "2026-08-19T00:00:40.000Z" } },
    };
    const doubled = collapseDoubledParts([part, part, part, part]);
    expect(doubled).toHaveLength(2);
    const times = doubled.flatMap((p) => sourceTimestampsMs(p));
    expect(elapsedMsFromSource(times, true, T40)).toBe(40_000);
  });
});

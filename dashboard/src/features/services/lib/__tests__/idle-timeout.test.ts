import { describe, it, expect } from "vitest";
import {
  planSleeps,
  idleTimeoutOptions,
  IDLE_TIMEOUT_PRESETS,
} from "@/features/services/lib/idle-timeout";

describe("planSleeps", () => {
  it("is true only for free/untiered plans (paid tiers stay always-on)", () => {
    expect(planSleeps(null)).toBe(true); // untiered bare CR defaults to free
    expect(planSleeps("free")).toBe(true);
    expect(planSleeps("starter")).toBe(false);
    expect(planSleeps("pro_plus")).toBe(false);
  });
});

describe("idleTimeoutOptions", () => {
  it("returns the presets when the current value is one of them", () => {
    expect(idleTimeoutOptions(900)).toEqual([...IDLE_TIMEOUT_PRESETS]);
  });

  it("includes a non-preset current value (set via API/CLI) so it isn't dropped", () => {
    const opts = idleTimeoutOptions(1234);
    expect(opts).toContain(1234);
    // sorted ascending, default (0) first
    expect(opts[0]).toBe(0);
    expect([...opts]).toEqual([...opts].sort((a, b) => a - b));
  });

  it("clamps a negative current value to 0 (no duplicate default)", () => {
    expect(idleTimeoutOptions(-5)).toEqual([...IDLE_TIMEOUT_PRESETS]);
  });
});

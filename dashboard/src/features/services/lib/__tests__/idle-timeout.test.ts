import { describe, it, expect } from "vitest";
import {
  autoSleepEligible,
  autoSleepEligibleType,
  planSleeps,
  idleTimeoutOptions,
  IDLE_TIMEOUT_PRESETS,
} from "@/features/services/lib/idle-timeout";

describe("auto-sleep eligibility", () => {
  it("only gives public web services an activator wake path", () => {
    expect(autoSleepEligibleType("web_service")).toBe(true);
    for (const type of [
      "private_service",
      "background_worker",
      "cron_job",
      "static_site",
    ]) {
      expect(autoSleepEligibleType(type)).toBe(false);
    }
  });

  it("also requires a free or untiered plan", () => {
    expect(autoSleepEligible("web_service", null)).toBe(true);
    expect(autoSleepEligible("web_service", "free")).toBe(true);
    expect(autoSleepEligible("web_service", "starter")).toBe(false);
    expect(autoSleepEligible("private_service", "free")).toBe(false);
  });
});

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

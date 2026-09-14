import { describe, expect, it } from "vitest";
import {
  supportsMaxShutdownDelay,
  supportsScaling,
  supportsScalingType,
} from "@/features/services/lib/service-type";
import type { ServiceView } from "@/features/services/types";

describe("supportsMaxShutdownDelay", () => {
  it.each([
    ["web_service", true],
    ["private_service", true],
    ["background_worker", true],
    ["cron_job", false],
    ["static_site", false],
  ])("returns %s => %s", (type, want) => {
    expect(supportsMaxShutdownDelay({ type } as ServiceView)).toBe(want);
  });
});

describe("supportsScaling (m101/t002)", () => {
  it.each([
    ["web_service", true],
    ["private_service", true],
    ["background_worker", true],
    ["cron_job", false],
    ["static_site", false],
  ])("returns %s => %s", (type, want) => {
    expect(supportsScaling({ type } as ServiceView)).toBe(want);
  });

  it.each([
    ["web", true],
    ["private", true],
    ["worker", true],
    ["cron", false],
    ["static", false],
    [null, false],
  ] as const)("type key %s => %s", (type, want) => {
    expect(supportsScalingType(type)).toBe(want);
  });
});

import { describe, expect, it } from "vitest";
import { supportsMaxShutdownDelay } from "@/features/services/lib/service-type";
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

import { describe, it, expect } from "vitest";
import type { LimitSummary } from "@/features/metrics/lib/limit-summary";
import {
  utilizationEmptyReason,
  type UtilizationEmptyReason,
} from "@/features/metrics/lib/utilization";

const NONE: LimitSummary = { kind: "none" };
const SINGLE: LimitSummary = { kind: "single", value: 512 };
const VARY: LimitSummary = { kind: "vary" };

describe("utilizationEmptyReason", () => {
  it.each<
    [
      string,
      {
        percentage: boolean;
        usage: boolean;
        limit: LimitSummary;
        runsInstances: boolean;
      },
      UtilizationEmptyReason | null,
    ]
  >([
    [
      "surviving percentages draw even with no current limit",
      { percentage: true, usage: true, limit: NONE, runsInstances: false },
      null,
    ],
    [
      "no usage is no data, whatever the limit",
      { percentage: false, usage: false, limit: SINGLE, runsInstances: true },
      "no-usage",
    ],
    [
      "no usage outranks a missing limit",
      { percentage: false, usage: false, limit: NONE, runsInstances: true },
      "no-usage",
    ],
    [
      "a running service with no limit is limitless",
      { percentage: false, usage: true, limit: NONE, runsInstances: true },
      "no-limit",
    ],
    [
      "a parked service's empty limit read is no pods, not no limit",
      { percentage: false, usage: true, limit: NONE, runsInstances: false },
      "percentage-unavailable",
    ],
    [
      "a limit with no surviving percentage is unavailable",
      { percentage: false, usage: true, limit: SINGLE, runsInstances: true },
      "percentage-unavailable",
    ],
    [
      "mixed limits with no surviving percentage are unavailable",
      { percentage: false, usage: true, limit: VARY, runsInstances: false },
      "percentage-unavailable",
    ],
  ])("%s", (_name, reads, reason) => {
    expect(utilizationEmptyReason(reads)).toBe(reason);
  });
});

import { readFileSync } from "node:fs";
import { describe, it, expect } from "vitest";
import {
  COMPUTE_TIER_COUNT,
  servicePlanSlotCount,
  formatInstanceCPU,
  formatInstanceMemory,
  legacyFreeNotice,
  offersFreeTier,
} from "@/features/services/lib/instance-type";

describe("formatInstanceCPU", () => {
  it("converts millicores to fractional cores", () => {
    expect(formatInstanceCPU("100m")).toBe("0.1 CPU");
    expect(formatInstanceCPU("500m")).toBe("0.5 CPU");
  });

  it("keeps whole-core values as whole numbers, not 1.0", () => {
    expect(formatInstanceCPU("1")).toBe("1 CPU");
    expect(formatInstanceCPU("2")).toBe("2 CPU");
    expect(formatInstanceCPU("8")).toBe("8 CPU");
  });
});

describe("formatInstanceMemory", () => {
  it("relabels Mi/Gi as MB/GB (Render's card unit spelling)", () => {
    expect(formatInstanceMemory("512Mi")).toBe("512 MB");
    expect(formatInstanceMemory("2Gi")).toBe("2 GB");
    expect(formatInstanceMemory("32Gi")).toBe("32 GB");
  });

  it("returns the raw string for an unrecognized unit rather than throwing", () => {
    expect(formatInstanceMemory("512")).toBe("512");
  });
});

describe("COMPUTE_TIER_COUNT (w4/329)", () => {
  // The create form's pending plan grid previews this many slots, so it must
  // be the real ladder: the compute tiers in lego/types/tiers/tiers.yaml.
  it("matches the compute tiers in tiers.yaml", () => {
    const yaml = readFileSync(
      `${process.cwd()}/../lego/types/tiers/tiers.yaml`,
      "utf8",
    );
    const compute = yaml.slice(
      yaml.indexOf("\ncompute:"),
      yaml.indexOf("\npostgres:"),
    );
    expect(compute.match(/^\s+- id: /gm)).toHaveLength(COMPUTE_TIER_COUNT);
    expect(servicePlanSlotCount("web_service")).toBe(COMPUTE_TIER_COUNT);
    expect(servicePlanSlotCount("private_service")).toBe(
      COMPUTE_TIER_COUNT - 1,
    );
  });
});

// w4/351: the Plan-page notice covers exactly the types Free was withdrawn
// from, while they are on Free or untiered.
describe("legacyFreeNotice", () => {
  it.each([
    "web_service",
    "private_service",
    "background_worker",
    "cron_job",
    "static_site",
  ])("is given to %s exactly when it cannot be offered Free", (type) => {
    const paidOnly = !offersFreeTier(type);
    expect(legacyFreeNotice(type, "free") !== null).toBe(paidOnly);
    expect(legacyFreeNotice(type, null) !== null).toBe(paidOnly);
  });

  it("names Free or untiered, and stays silent for a paid or unknown plan", () => {
    expect(legacyFreeNotice("cron_job", "free")).toBe(
      "services.planPickerLegacyFreeCronJob",
    );
    expect(legacyFreeNotice("cron_job", null)).toBe(
      "services.planPickerUntieredCronJob",
    );
    expect(legacyFreeNotice("cron_job", "starter")).toBeNull();
    expect(legacyFreeNotice("cron_job", undefined)).toBeNull();
    expect(legacyFreeNotice(null, "free")).toBeNull();
  });
});

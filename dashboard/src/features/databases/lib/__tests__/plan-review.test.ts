import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  centsToUsd,
  databaseReviewPricing,
  usdToCents,
} from "@/features/databases/lib/plan-review";

// The prices come from bex's own sheet, as bex-api serves them to the plan
// catalogue (`monthlyUsd`, `storageMonthlyUsdPerGB`): one pricing month of a
// tier's instance rate, and of the storage rate per GB, formatted to cents as
// pricing.FormatUSD does. Nothing here copies a figure from Render.
const SHEET = readFileSync(
  `${process.cwd()}/../lego/backend/internal/pricing/pricing.yaml`,
  "utf8",
);
const MONTH_SECONDS = 2_628_000;
function sheetMonthlyUsd(section: string, tier: string): string {
  const block = SHEET.slice(SHEET.indexOf(`\n${section}:\n`));
  const rate = new RegExp(`- tier: ${tier}\\n\\s+usdPerSecond: ([\\d.]+)`).exec(
    block,
  );
  if (!rate) throw new Error(`pricing.yaml has no ${section} tier ${tier}`);
  return (Number(rate[1]) * MONTH_SECONDS).toFixed(2);
}
const storage = /\nstorage:\n\s+usdPerGBSecond: ([\d.]+)/.exec(SHEET);
if (!storage) throw new Error("pricing.yaml has no storage rate");
const STORAGE_PER_GB = (Number(storage[1]) * MONTH_SECONDS).toFixed(2);

// A ≥1-CPU plan, the first that can run high availability.
const PLAN = sheetMonthlyUsd("postgres", "1c-2g");
const P = usdToCents(PLAN)!;
const R = Number(STORAGE_PER_GB);

describe("databaseReviewPricing (w3/m674, Render's WP)", () => {
  it("reads the sheet's figures", () => {
    // Guards the parse above: a paid plan and the storage rate are priced.
    expect(P).toBeGreaterThan(0);
    expect(R).toBeGreaterThan(0);
  });

  it("prices one instance: the plan and its storage", () => {
    const pricing = databaseReviewPricing({
      planMonthlyUsd: PLAN,
      storageMonthlyUsdPerGB: STORAGE_PER_GB,
      highAvailability: false,
      readReplicas: 0,
      storageGB: 10,
    });
    expect(pricing).toEqual({
      billedInstances: 1,
      planCents: P,
      highAvailabilityCents: 0,
      readReplicasCents: 0,
      storageCents: Math.round(10 * R * 100),
      totalCents: P + Math.round(10 * R * 100),
    });
  });

  it("prices the HA standby like the primary, and its storage too", () => {
    const pricing = databaseReviewPricing({
      planMonthlyUsd: PLAN,
      storageMonthlyUsdPerGB: STORAGE_PER_GB,
      highAvailability: true,
      readReplicas: 0,
      storageGB: 10,
    });
    expect(pricing.billedInstances).toBe(2);
    expect(pricing.highAvailabilityCents).toBe(P);
    expect(pricing.storageCents).toBe(Math.round(10 * 2 * R * 100));
    expect(pricing.totalCents).toBe(2 * P + Math.round(10 * 2 * R * 100));
  });

  it("prices every read replica beside the standby", () => {
    const pricing = databaseReviewPricing({
      planMonthlyUsd: PLAN,
      storageMonthlyUsdPerGB: STORAGE_PER_GB,
      highAvailability: true,
      readReplicas: 2,
      storageGB: 25,
    });
    expect(pricing.billedInstances).toBe(4);
    expect(pricing.readReplicasCents).toBe(2 * P);
    expect(pricing.storageCents).toBe(Math.round(25 * 4 * R * 100));
    // The lines add up to the total they show.
    expect(pricing.totalCents).toBe(
      pricing.planCents! +
        pricing.highAvailabilityCents! +
        pricing.readReplicasCents! +
        pricing.storageCents!,
    );
    expect(pricing.totalCents).toBe(4 * P + Math.round(25 * 4 * R * 100));
  });

  it("prices a storage-only change at the new size on the current plan", () => {
    const before = databaseReviewPricing({
      planMonthlyUsd: PLAN,
      storageMonthlyUsdPerGB: STORAGE_PER_GB,
      highAvailability: true,
      readReplicas: 1,
      storageGB: 10,
    });
    const after = databaseReviewPricing({
      planMonthlyUsd: PLAN,
      storageMonthlyUsdPerGB: STORAGE_PER_GB,
      highAvailability: true,
      readReplicas: 1,
      storageGB: 15,
    });
    // Five more GB on each of three instances; compute is unchanged.
    expect(after.totalCents! - before.totalCents!).toBe(
      Math.round(5 * 3 * R * 100),
    );
    expect(after.planCents).toBe(before.planCents);
  });

  it("lists no storage on a Free plan, whose storage is included", () => {
    const pricing = databaseReviewPricing({
      planMonthlyUsd: sheetMonthlyUsd("postgres", "free"),
      storageMonthlyUsdPerGB: "0.00",
      highAvailability: false,
      readReplicas: 0,
      storageGB: null,
    });
    expect(pricing.storageCents).toBeNull();
    expect(pricing.totalCents).toBe(0);
  });

  it("totals nothing for a plan the sheet does not price", () => {
    const pricing = databaseReviewPricing({
      planMonthlyUsd: "",
      storageMonthlyUsdPerGB: STORAGE_PER_GB,
      highAvailability: true,
      readReplicas: 1,
      storageGB: 10,
    });
    expect(pricing.planCents).toBeNull();
    expect(pricing.highAvailabilityCents).toBeNull();
    expect(pricing.readReplicasCents).toBeNull();
    expect(pricing.totalCents).toBeNull();
    // The storage line still has its own price.
    expect(pricing.storageCents).toBe(Math.round(10 * 3 * R * 100));
  });

  it("totals nothing when the storage rate is missing", () => {
    const pricing = databaseReviewPricing({
      planMonthlyUsd: PLAN,
      storageMonthlyUsdPerGB: "",
      highAvailability: false,
      readReplicas: 0,
      storageGB: 10,
    });
    expect(pricing.storageCents).toBeNull();
    expect(pricing.totalCents).toBeNull();
  });

  it("rounds a fractional rate once per line, so cents add up", () => {
    // A rate with three significant digits, the shape of a disk rate.
    const pricing = databaseReviewPricing({
      planMonthlyUsd: "1.00",
      storageMonthlyUsdPerGB: "0.175",
      highAvailability: true,
      readReplicas: 0,
      storageGB: 3,
    });
    // 3 GB × 2 instances × $0.175 = $1.05.
    expect(pricing.storageCents).toBe(105);
    expect(pricing.totalCents).toBe(305);
  });
});

describe("usdToCents / centsToUsd", () => {
  it("reads bex-api's decimals and writes them back", () => {
    expect(usdToCents("28.00")).toBe(2800);
    expect(usdToCents("0.21")).toBe(21);
    expect(usdToCents("")).toBeNull();
    expect(usdToCents("n/a")).toBeNull();
    expect(usdToCents(undefined)).toBeNull();
    expect(centsToUsd(945)).toBe("9.45");
    expect(centsToUsd(2800)).toBe("28.00");
  });
});

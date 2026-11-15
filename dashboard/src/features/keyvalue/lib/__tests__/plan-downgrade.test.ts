import { describe, expect, it } from "vitest";
import {
  isPlanDowngrade,
  tierMemoryBytes,
} from "@/features/keyvalue/lib/plan-downgrade";

const tier = (memory: string) => ({ memory });

describe("tierMemoryBytes", () => {
  it("reads the catalog's binary k8s quantity spellings", () => {
    expect(tierMemoryBytes("128Mi")).toBe(128 * 1024 ** 2);
    expect(tierMemoryBytes("1Gi")).toBe(1024 ** 3);
    expect(tierMemoryBytes("1.5Gi")).toBe(1.5 * 1024 ** 3);
    expect(tierMemoryBytes(" 3Gi ")).toBe(3 * 1024 ** 3);
  });

  it("returns null for a spelling it cannot compare", () => {
    expect(tierMemoryBytes("")).toBeNull();
    expect(tierMemoryBytes("1G")).toBeNull();
    expect(tierMemoryBytes("large")).toBeNull();
  });
});

describe("isPlanDowngrade", () => {
  // w8/249: bex-api refuses a move to a smaller plan, comparing memory
  // (keyvalue.ErrPlanDowngrade) — the same numbers the catalog sends.
  it("is true only for a tier with less memory than the current one", () => {
    expect(isPlanDowngrade(tier("128Mi"), tier("1Gi"))).toBe(true);
    expect(isPlanDowngrade(tier("1Gi"), tier("1Gi"))).toBe(false);
    expect(isPlanDowngrade(tier("3Gi"), tier("1Gi"))).toBe(false);
  });

  // The rule is memory, not list position: a catalog that is not ordered
  // smallest-first still finds every smaller plan.
  it("does not depend on the catalog's order", () => {
    expect(isPlanDowngrade(tier("256Mi"), tier("3Gi"))).toBe(true);
    expect(isPlanDowngrade(tier("5Gi"), tier("3Gi"))).toBe(false);
  });

  it("leaves the card selectable when either side cannot be compared", () => {
    expect(isPlanDowngrade(tier("128Mi"), undefined)).toBe(false);
    expect(isPlanDowngrade(tier("128Mi"), tier("large"))).toBe(false);
    expect(isPlanDowngrade(tier("large"), tier("1Gi"))).toBe(false);
  });
});

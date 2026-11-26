import { describe, expect, it } from "vitest";
import {
  almostFullAction,
  storageAlmostFull,
} from "@/features/databases/lib/storage";

// w3/m656: Render's almost-full rule, `storageUsedPercent > 80` with less
// than 5 GiB left of `storageTotal` (docs/render-artifacts/postgres-dashboard.md
// § Almost-full warning).

const GIB = 1024 ** 3;
const usage = (storageUsedPercent: number | null, totalGiB: number | null) => ({
  storageUsedPercent,
  storageTotal: totalGiB == null ? null : String(Math.round(totalGiB * GIB)),
});

describe("storageAlmostFull", () => {
  it.each([
    ["80% exactly", usage(80, 10), false],
    ["81% of 10 GiB", usage(81, 10), true],
    ["81% with 4.9 GiB left", usage(81, 4.9 / 0.19), true],
    ["81% with 5.1 GiB left", usage(81, 5.1 / 0.19), false],
    ["95% of 1 TiB, 51 GiB left", usage(95, 1024), false],
    ["99.9% of 1 TiB, 1 GiB left", usage(99.9, 1024), true],
    // A database below the physical volume's floor can write past its size.
    ["past 100% of 1 GiB", usage(140, 1), true],
    ["no usage", usage(null, 10), false],
    ["no total", usage(90, null), false],
    ["a zero total", usage(90, 0), false],
  ])("%s → %s", (_name, value, want) => {
    expect(storageAlmostFull(value)).toBe(want);
  });

  it("is false before the usage is read", () => {
    expect(storageAlmostFull(null)).toBe(false);
    expect(
      storageAlmostFull({ storageUsedPercent: 90, storageTotal: "lots" }),
    ).toBe(false);
  });
});

describe("almostFullAction", () => {
  const paid = { storageGB: 10, maxStorageGB: 16384 };

  it("grows storage that can grow", () => {
    expect(almostFullAction({ diskSizeGB: 10 }, paid)).toBe("increaseStorage");
  });

  it("upgrades a plan whose storage is fixed", () => {
    expect(
      almostFullAction({ diskSizeGB: 1 }, { storageGB: 1, maxStorageGB: 1 }),
    ).toBe("upgradePlan");
  });

  it("asks support once the storage is at its plan's maximum", () => {
    expect(almostFullAction({ diskSizeGB: 16384 }, paid)).toBe("support");
  });

  it("offers the storage control while the plan is unread", () => {
    expect(almostFullAction({ diskSizeGB: 1 }, undefined)).toBe(
      "increaseStorage",
    );
  });
});

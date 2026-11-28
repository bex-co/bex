import { afterEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import i18n from "@/i18n/init";
import { useKeyValueInstanceTypes } from "@/features/keyvalue/hooks/use-key-value-instance-types";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

function tier(id: string, name: string, available: boolean) {
  return {
    __typename: "KeyValueInstanceType" as const,
    id,
    name,
    cpu: "1",
    memory: "1Gi",
    storageGB: 1,
    monthlyUsd: "10.00",
    storageMonthlyUsdPerGB: "0.21",
    available,
    unavailableReason: available ? "" : "english only",
  };
}

// w4/552: bex-api's reason for a paused plan is English only, so the
// dashboard composes it in its own language, naming the largest plan left.
describe("useKeyValueInstanceTypes paused-plan reason", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it.each([
    [
      "en",
      'Key Value plan "Pro Ultra" is temporarily unavailable; choose Pro Max or a smaller plan.',
    ],
    [
      "zh",
      "Key Value 实例规格“Pro Ultra”暂不可用，请选择 Pro Max 或更小的规格。",
    ],
  ])("reads in %s", async (language, want) => {
    await i18n.changeLanguage(language);
    mockUseQuery.mockReturnValue({
      data: {
        keyValueInstanceTypes: [
          tier("starter", "Starter", true),
          tier("pro_max", "Pro Max", true),
          tier("pro_ultra", "Pro Ultra", false),
        ],
      },
      loading: false,
      error: undefined,
    });
    const { result } = renderHook(() => useKeyValueInstanceTypes());
    const [starter, , ultra] = result.current.instanceTypes;
    expect(ultra.unavailableReason).toBe(want);
    expect(starter.unavailableReason).toBe("");
  });
});

// The review's Storage line rate comes from the catalog too (w3/m674): bex
// bills Key Value storage, so the hook passes the sheet rate through, and a
// catalog from an older API that sends none reads "" (unknown, never $0).
describe("useKeyValueInstanceTypes storage rate", () => {
  it("passes the rate through and defaults a missing one to unknown", () => {
    mockUseQuery.mockReturnValue({
      data: {
        keyValueInstanceTypes: [
          tier("starter", "Starter", true),
          {
            ...tier("legacy", "Legacy", true),
            storageMonthlyUsdPerGB: undefined,
          },
        ],
      },
      loading: false,
      error: undefined,
    });
    const { result } = renderHook(() => useKeyValueInstanceTypes());
    expect(
      result.current.instanceTypes.map((it) => [
        it.id,
        it.storageMonthlyUsdPerGB,
      ]),
    ).toEqual([
      ["starter", "0.21"],
      ["legacy", ""],
    ]);
  });
});

// w6/m459: each plan's connection limit comes from the catalog, never a
// frontend ladder; a catalog that sends none (or a non-positive figure) leaves
// it undefined, so no card reads "0 connections".
describe("useKeyValueInstanceTypes connection limit", () => {
  it("passes each plan's limit through and drops a missing or zero one", () => {
    const withLimit = (id: string, connectionLimit?: number) => ({
      ...tier(id, id, true),
      ...(connectionLimit === undefined ? {} : { connectionLimit }),
    });
    mockUseQuery.mockReturnValue({
      data: {
        keyValueInstanceTypes: [
          withLimit("free", 50),
          withLimit("standard", 1000),
          withLimit("older-api"),
          withLimit("zero", 0),
        ],
      },
      loading: false,
      error: undefined,
    });
    const { result } = renderHook(() => useKeyValueInstanceTypes());
    expect(
      result.current.instanceTypes.map((it) => [it.id, it.connectionLimit]),
    ).toEqual([
      ["free", 50],
      ["standard", 1000],
      ["older-api", undefined],
      ["zero", undefined],
    ]);
  });
});

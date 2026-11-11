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

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import i18n from "@/i18n/init";
import { useInstanceTypes } from "@/features/services/hooks/use-instance-types";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

beforeEach(() => mockUseQuery.mockReset());

const CATALOG = [
  {
    __typename: "InstanceType" as const,
    id: "free",
    name: "Free",
    cpu: "100m",
    memory: "512Mi",
    monthlyUsd: "0.00",
    usdPerMinute: "0",
    available: true,
    unavailableReason: "",
  },
  {
    __typename: "InstanceType" as const,
    id: "standard",
    name: "Standard",
    cpu: "1",
    memory: "2Gi",
    monthlyUsd: "17.50",
    usdPerMinute: "0.0004",
    available: false,
    unavailableReason: "paused",
  },
];

describe("useInstanceTypes", () => {
  it("maps the wire InstanceType list, filtering out entries with no id", () => {
    mockUseQuery.mockReturnValue({
      data: { instanceTypes: [...CATALOG, null, { id: null, name: "bad" }] },
      loading: false,
      error: undefined,
    });

    const { result } = renderHook(() => useInstanceTypes());

    expect(result.current.instanceTypes).toEqual([
      {
        id: "free",
        name: "Free",
        cpu: "100m",
        memory: "512Mi",
        monthlyUsd: "0.00",
        usdPerMinute: "0",
        available: true,
        unavailableReason: "",
      },
      {
        id: "standard",
        name: "Standard",
        cpu: "1",
        memory: "2Gi",
        monthlyUsd: "17.50",
        usdPerMinute: "0.0004",
        available: false,
        unavailableReason:
          '"Standard" is temporarily unavailable; choose Free or a smaller instance type.',
      },
    ]);
  });

  // w4/552: bex-api's reason is English only, so the dashboard composes it in
  // its own language from the paused tier and the largest one still offered.
  describe("a paused tier's reason", () => {
    afterEach(async () => {
      await i18n.changeLanguage("en");
    });

    it("is in the dashboard's language", async () => {
      await i18n.changeLanguage("zh");
      mockUseQuery.mockReturnValue({
        data: { instanceTypes: CATALOG },
        loading: false,
        error: undefined,
      });
      const { result } = renderHook(() => useInstanceTypes());
      expect(result.current.byID("standard")?.unavailableReason).toBe(
        "“Standard”暂不可用，请选择 Free 或更小的实例类型。",
      );
      expect(result.current.byID("free")?.unavailableReason).toBe("");
    });

    it("keeps bex-api's reason when no tier is left to suggest", () => {
      mockUseQuery.mockReturnValue({
        data: { instanceTypes: [CATALOG[1]] },
        loading: false,
        error: undefined,
      });
      const { result } = renderHook(() => useInstanceTypes());
      expect(result.current.byID("standard")?.unavailableReason).toBe("paused");
    });
  });

  it("returns an empty list (not a crash) while loading with no data yet", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
    });
    const { result } = renderHook(() => useInstanceTypes());
    expect(result.current.instanceTypes).toEqual([]);
    expect(result.current.loading).toBe(true);
  });

  it("byID finds a tier by its Render plan id and is undefined for an unknown/null plan", () => {
    mockUseQuery.mockReturnValue({
      data: { instanceTypes: CATALOG },
      loading: false,
      error: undefined,
    });
    const { result } = renderHook(() => useInstanceTypes());

    expect(result.current.byID("standard")?.name).toBe("Standard");
    expect(result.current.byID("gold")).toBeUndefined();
    expect(result.current.byID(null)).toBeUndefined();
    expect(result.current.byID(undefined)).toBeUndefined();
  });

  it("surfaces a query error (e.g. instanceTypes unshipped on the server)", () => {
    const err = new Error('Cannot query field "instanceTypes"');
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: false,
      error: err,
    });
    const { result } = renderHook(() => useInstanceTypes());
    expect(result.current.error).toBe(err);
    expect(result.current.instanceTypes).toEqual([]);
  });
});

import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
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
  },
  {
    __typename: "InstanceType" as const,
    id: "standard",
    name: "Standard",
    cpu: "1",
    memory: "2Gi",
    monthlyUsd: "17.50",
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
      },
      {
        id: "standard",
        name: "Standard",
        cpu: "1",
        memory: "2Gi",
        monthlyUsd: "17.50",
      },
    ]);
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

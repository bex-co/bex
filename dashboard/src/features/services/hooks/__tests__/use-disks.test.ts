import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("@/common/hooks/use-translations", () => ({
  useTranslations: () => ({ t: (k: string) => k }),
}));

import { useDisk, useDiskSnapshots } from "@/features/services/hooks/use-disks";

beforeEach(() => {
  vi.clearAllMocks();
  mockUseMutation.mockReturnValue([vi.fn()]);
});

// The Disk tab has nothing to render for a diskless service except its add
// form, so the caller shows a skeleton while `loading`. This query is
// cache-and-network AND polls, so Apollo raises `loading` again on every
// background refetch — which swapped the open add form for a skeleton and
// destroyed the mount path the tenant was typing.
//
// Caught on production 2026-08-24: the form vanished ~2s after opening, with
// the skeleton appearing in the same frame. `loading` must therefore mean
// "nothing to show yet", not "a request is in flight".
describe("useDisk loading semantics", () => {
  it("reports loading before the first result arrives", () => {
    mockUseQuery.mockReturnValue({ data: undefined, loading: true, error: undefined, refetch: vi.fn() });

    const { result } = renderHook(() => useDisk("srv-1"));

    expect(result.current.loading).toBe(true);
    expect(result.current.disk).toBeNull();
  });

  it("does NOT report loading during a background refetch of a diskless service", () => {
    // Apollo's shape mid-poll: data already delivered, request in flight again.
    mockUseQuery.mockReturnValue({ data: { disks: [] }, loading: true, error: undefined, refetch: vi.fn() });

    const { result } = renderHook(() => useDisk("srv-1"));

    // An empty list is a real answer — "this service has no disk" — not a
    // pending one. Reporting loading here is what unmounted the add form.
    expect(result.current.loading).toBe(false);
    expect(result.current.disk).toBeNull();
  });

  it("does NOT report loading during a background refetch of a service with a disk", () => {
    mockUseQuery.mockReturnValue({
      data: { disks: [{ id: "dsk-1", name: "data", mountPath: "/var/data", sizeGB: 10, serviceId: "srv-1" }] },
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useDisk("srv-1"));

    expect(result.current.loading).toBe(false);
    expect(result.current.disk?.mountPath).toBe("/var/data");
  });

  // GraphQL marks every field nullable; the hook is the boundary that asserts
  // a well-formed disk so no component has to defend against a null mount path.
  it("normalizes a partially-null disk rather than passing nulls through", () => {
    mockUseQuery.mockReturnValue({
      data: { disks: [{ id: null, name: null, mountPath: null, sizeGB: null, serviceId: null }] },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useDisk("srv-1"));

    expect(result.current.disk).toEqual({ id: "", name: "", mountPath: "", sizeGB: 0, serviceId: "" });
  });
});

describe("useDiskSnapshots loading semantics", () => {
  it("does NOT report loading during a background refetch, so the list never flashes a skeleton", () => {
    mockUseQuery.mockReturnValue({
      data: { diskSnapshots: [{ createdAt: "2026-08-24T02:00:00Z", snapshotKey: "k1", instanceId: "srv-1" }] },
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useDiskSnapshots("dsk-1"));

    expect(result.current.loading).toBe(false);
    expect(result.current.snapshots).toHaveLength(1);
  });

  it("drops a snapshot with no key rather than rendering an unrestorable row", () => {
    mockUseQuery.mockReturnValue({
      data: { diskSnapshots: [{ createdAt: "2026-08-24T02:00:00Z", snapshotKey: null, instanceId: "srv-1" }] },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useDiskSnapshots("dsk-1"));

    // snapshotKey is the argument to restore; a row without one is a button
    // that cannot work.
    expect(result.current.snapshots).toEqual([]);
  });
});

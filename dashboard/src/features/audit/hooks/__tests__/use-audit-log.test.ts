import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";

const mockUseQuery = vi.fn();
const mockClientQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useApolloClient: () => ({ query: mockClientQuery }),
}));

import { useAuditLog } from "@/features/audit/hooks/use-audit-log";

function rawEvent(id: string, overrides: Partial<Record<string, unknown>> = {}) {
  return {
    __typename: "AuditLog" as const,
    id,
    timestamp: "2026-07-11T00:00:00Z",
    actor: "user:alice",
    actorMethod: "session",
    action: "update",
    status: "success",
    resource: "workspace:tea-1",
    ...overrides,
  };
}

function page(n: number, prefix = "ev") {
  return Array.from({ length: n }, (_, i) => rawEvent(`${prefix}-${i}`));
}

beforeEach(() => {
  mockUseQuery.mockReset();
  mockClientQuery.mockReset();
});

describe("useAuditLog", () => {
  it("populates events from the initial query", () => {
    mockUseQuery.mockReturnValue({
      data: { auditLogs: page(3) },
      loading: false,
      error: undefined,
    });

    const { result } = renderHook(() => useAuditLog("tea-1"));

    expect(result.current.events).toHaveLength(3);
    expect(result.current.events[0]).toMatchObject({ id: "ev-0", status: "success" });
    expect(result.current.forbidden).toBe(false);
    expect(result.current.unavailable).toBe(false);
    expect(result.current.error).toBeUndefined();
  });

  it("loadMore appends the next page and dedupes any overlap at the boundary", async () => {
    // A full first page (20 = PAGE_SIZE) so hasMore starts true.
    mockUseQuery.mockReturnValue({
      data: { auditLogs: page(20) },
      loading: false,
      error: undefined,
    });
    // The next page overlaps on "ev-19" (the last id of page one) — the hook
    // should not double-count it.
    mockClientQuery.mockResolvedValue({
      data: { auditLogs: [rawEvent("ev-19"), ...page(4, "ev2")] },
    });

    const { result } = renderHook(() => useAuditLog("tea-1"));
    expect(result.current.hasMore).toBe(true);

    act(() => {
      result.current.loadMore();
    });

    await waitFor(() => expect(result.current.events).toHaveLength(24));
    const ids = result.current.events.map((e) => e.id);
    expect(new Set(ids).size).toBe(ids.length); // no duplicates
    expect(mockClientQuery).toHaveBeenCalledWith(
      expect.objectContaining({ variables: expect.objectContaining({ cursor: "ev-19" }) }),
    );
    // The appended page (4 items, after dedup) is short of PAGE_SIZE.
    expect(result.current.hasMore).toBe(false);
  });

  it("a forbidden (403) response sets forbidden with no error", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: false,
      error: new Error("forbidden"),
    });

    const { result } = renderHook(() => useAuditLog("tea-1"));

    expect(result.current.forbidden).toBe(true);
    expect(result.current.unavailable).toBe(false);
    expect(result.current.error).toBeUndefined();
  });

  it("an unavailable (503, store not configured) response sets unavailable with no error", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: false,
      error: new Error("audit log store not configured"),
    });

    const { result } = renderHook(() => useAuditLog("tea-1"));

    expect(result.current.unavailable).toBe(true);
    expect(result.current.forbidden).toBe(false);
    expect(result.current.error).toBeUndefined();
  });

  it("any other failure sets a generic error", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: false,
      error: new Error("boom"),
    });

    const { result } = renderHook(() => useAuditLog("tea-1"));

    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.forbidden).toBe(false);
    expect(result.current.unavailable).toBe(false);
  });
});

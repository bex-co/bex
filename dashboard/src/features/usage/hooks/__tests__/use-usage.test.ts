import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useUsage } from "../use-usage";
import {
  createLoadingQueryResult,
  createErrorQueryResult,
  createSuccessQueryResult,
} from "@/test/mocks/apollo";

const mockUseQuery = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

describe("useUsage", () => {
  beforeEach(() => {
    mockUseQuery.mockReset();
  });

  it("returns null summary and loading=true while the query is in-flight", () => {
    mockUseQuery.mockReturnValue(createLoadingQueryResult());

    const { result } = renderHook(() => useUsage());

    expect(result.current.summary).toBeNull();
    expect(result.current.loading).toBe(true);
    expect(result.current.error).toBeUndefined();
  });

  it("maps the workspace-scoped usage response into a typed summary", () => {
    mockUseQuery.mockReturnValue(
      createSuccessQueryResult({
        usage: {
          workspaceId: "ws-abc123",
          services: [
            {
              serviceId: "eden-cms-v2",
              rows: [
                { kind: "instance_seconds", tier: "starter", total: 7200 },
                { kind: "egress_bytes", tier: "", total: 1048576 },
              ],
            },
            {
              serviceId: "nightly-report",
              rows: [{ kind: "build_seconds", tier: "", total: 600 }],
            },
          ],
        },
      }),
    );

    const { result } = renderHook(() => useUsage());

    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeUndefined();
    expect(result.current.summary).toEqual({
      workspaceId: "ws-abc123",
      services: [
        {
          serviceId: "eden-cms-v2",
          rows: [
            { kind: "instance_seconds", tier: "starter", total: 7200 },
            { kind: "egress_bytes", tier: "", total: 1048576 },
          ],
        },
        {
          serviceId: "nightly-report",
          rows: [{ kind: "build_seconds", tier: "", total: 600 }],
        },
      ],
    });
  });

  it("returns null summary when usage is null (empty workspace / no metered period)", () => {
    mockUseQuery.mockReturnValue(
      createSuccessQueryResult({ usage: null }),
    );

    const { result } = renderHook(() => useUsage());

    expect(result.current.summary).toBeNull();
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeUndefined();
  });

  it("filters out null rows and null services from a partially-null response", () => {
    mockUseQuery.mockReturnValue(
      createSuccessQueryResult({
        usage: {
          workspaceId: "ws-xyz",
          services: [
            null,
            {
              serviceId: "api",
              rows: [null, { kind: "instance_seconds", tier: "hobby", total: 3600 }],
            },
          ],
        },
      }),
    );

    const { result } = renderHook(() => useUsage());

    expect(result.current.summary?.services).toHaveLength(1);
    expect(result.current.summary?.services[0].rows).toHaveLength(1);
    expect(result.current.summary?.services[0].rows[0].tier).toBe("hobby");
  });

  it("surfaces a query error as error, leaving summary null", () => {
    mockUseQuery.mockReturnValue(
      createErrorQueryResult("network error"),
    );

    const { result } = renderHook(() => useUsage());

    expect(result.current.summary).toBeNull();
    expect(result.current.loading).toBe(false);
    expect(result.current.error?.message).toBe("network error");
  });

  it("polls every 60 seconds with cache-and-network fetchPolicy", () => {
    mockUseQuery.mockReturnValue(createLoadingQueryResult());

    renderHook(() => useUsage());

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        pollInterval: 60_000,
        fetchPolicy: "cache-and-network",
      }),
    );
  });
});

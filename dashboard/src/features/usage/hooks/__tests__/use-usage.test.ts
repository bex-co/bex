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

// The queried workspace is the switcher's selection (w6/m18), never one the
// hook resolves itself — same seam useCreateService/useServices use.
let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

describe("useUsage", () => {
  beforeEach(() => {
    mockUseQuery.mockReset();
    currentWorkspaceId = "tea-1";
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
          estimatedCost: {
            totalUsd: "4.92",
            resources: [
              {
                serviceId: "srv-cms",
                serviceName: "eden-cms-v2",
                resourceKind: "service",
                costUsd: "4.90",
                charges: [
                  {
                    kind: "instance_seconds",
                    tier: "starter",
                    unit: "hr",
                    rateUsd: "0.006713",
                    quantity: "730.00",
                    costUsd: "4.90",
                  },
                  {
                    kind: "egress_bytes",
                    tier: "",
                    unit: "GB",
                    rateUsd: "0.015",
                    quantity: "1.00",
                    costUsd: "0.02",
                  },
                ],
              },
              {
                serviceId: "nightly-report",
                resourceKind: "service",
                costUsd: "0.00",
                charges: [],
              },
            ],
          },
        },
      }),
    );

    const { result } = renderHook(() => useUsage());

    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeUndefined();
    expect(result.current.summary).toEqual({
      workspaceId: "ws-abc123",
      period: "",
      // coverage absent from the response ⇒ the honest "unknown" default (w4/048).
      coverage: { state: "unknown", through: "", degradedSources: [] },
      estimatedCost: {
        totalUsd: "4.92",
        resources: [
          {
            serviceId: "srv-cms",
            serviceName: "eden-cms-v2",
            resourceKind: "service",
            costUsd: "4.90",
            charges: [
              {
                kind: "instance_seconds",
                tier: "starter",
                unit: "hr",
                rateUsd: "0.006713",
                quantity: "730.00",
                costUsd: "4.90",
              },
              {
                kind: "egress_bytes",
                tier: "",
                unit: "GB",
                rateUsd: "0.015",
                quantity: "1.00",
                costUsd: "0.02",
              },
            ],
          },
          {
            // serviceName absent from the response maps to "" (id fallback).
            serviceId: "nightly-report",
            serviceName: "",
            resourceKind: "service",
            costUsd: "0.00",
            charges: [],
          },
        ],
      },
      // billing absent from the response ⇒ null (estimate-only, m48).
      billing: null,
    });
  });

  it("maps a partial coverage read so the page can caveat the estimate (w4/048)", () => {
    mockUseQuery.mockReturnValue(
      createSuccessQueryResult({
        usage: {
          workspaceId: "ws",
          coverage: {
            state: "partial",
            through: "2026-09-01T00:00:00Z",
            degradedSources: ["direct", "http", "instance"],
          },
          estimatedCost: { totalUsd: "2.45", resources: [] },
        },
      }),
    );

    const { result } = renderHook(() => useUsage());

    expect(result.current.summary?.coverage).toEqual({
      state: "partial",
      through: "2026-09-01T00:00:00Z",
      degradedSources: ["direct", "http", "instance"],
    });
  });

  it("maps real billing (current cost + finalized invoices) when present (m48)", () => {
    mockUseQuery.mockReturnValue(
      createSuccessQueryResult({
        usage: {
          workspaceId: "ws-abc123",
          billing: {
            currentCost: {
              amountUsd: "12.34",
              creditsAppliedUsd: "10.00",
              amountDueUsd: "2.34",
              currency: "USD",
              periodStart: "2026-07-01T00:00:00Z",
              periodEnd: "2026-07-20T00:00:00Z",
            },
            invoices: [
              {
                id: "inv_1",
                status: "FINALIZED",
                amountUsd: "40.00",
                // The credit figures are deliberately absent here: a server
                // that predates them must map to "0.00", never undefined.
                currency: "USD",
                periodStart: "2026-06-01T00:00:00Z",
                periodEnd: "2026-07-01T00:00:00Z",
              },
              null, // a null invoice is filtered out
            ],
            credits: {
              availableUsd: "25.00",
              currency: "USD",
              grants: [
                {
                  name: "welcome",
                  remainingUsd: "25.00",
                  expiresAt: "2026-11-15T00:00:00Z",
                },
              ],
            },
          },
        },
      }),
    );

    const { result } = renderHook(() => useUsage());

    expect(result.current.summary?.billing).toEqual({
      currentCost: {
        amountUsd: "12.34",
        creditsAppliedUsd: "10.00",
        amountDueUsd: "2.34",
        currency: "USD",
        periodStart: "2026-07-01T00:00:00Z",
        periodEnd: "2026-07-20T00:00:00Z",
      },
      invoices: [
        {
          id: "inv_1",
          status: "FINALIZED",
          amountUsd: "40.00",
          creditsAppliedUsd: "0.00",
          amountDueUsd: "0.00",
          currency: "USD",
          periodStart: "2026-06-01T00:00:00Z",
          periodEnd: "2026-07-01T00:00:00Z",
        },
      ],
      credits: {
        availableUsd: "25.00",
        currency: "USD",
        grants: [
          {
            name: "welcome",
            remainingUsd: "25.00",
            expiresAt: "2026-11-15T00:00:00Z",
          },
        ],
      },
    });
  });

  it("returns null summary when usage is null (empty workspace / no metered period)", () => {
    mockUseQuery.mockReturnValue(createSuccessQueryResult({ usage: null }));

    const { result } = renderHook(() => useUsage());

    expect(result.current.summary).toBeNull();
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeUndefined();
  });

  it("filters out null charges and null resources from a partially-null response", () => {
    mockUseQuery.mockReturnValue(
      createSuccessQueryResult({
        usage: {
          workspaceId: "ws-abc123",
          estimatedCost: {
            totalUsd: "1.00",
            resources: [
              null,
              {
                serviceId: "srv-a",
                resourceKind: "service",
                costUsd: "1.00",
                charges: [
                  null,
                  {
                    kind: "instance_seconds",
                    tier: "hobby",
                    unit: "hr",
                    rateUsd: "1.00",
                    quantity: "1.00",
                    costUsd: "1.00",
                  },
                ],
              },
            ],
          },
        },
      }),
    );

    const { result } = renderHook(() => useUsage());

    const resources = result.current.summary?.estimatedCost?.resources;
    expect(resources).toHaveLength(1);
    expect(resources![0]!.charges).toHaveLength(1);
    expect(resources![0]!.charges[0]!.tier).toBe("hobby");
  });

  it("surfaces a query error as error, leaving summary null", () => {
    mockUseQuery.mockReturnValue(createErrorQueryResult("network error"));

    const { result } = renderHook(() => useUsage());

    expect(result.current.summary).toBeNull();
    expect(result.current.loading).toBe(false);
    expect(result.current.error?.message).toBe("network error");
  });

  it("polls every 60 seconds with the primed cache-first fetch policy", () => {
    mockUseQuery.mockReturnValue(createLoadingQueryResult());

    renderHook(() => useUsage());

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        pollInterval: 60_000,
        fetchPolicy: "cache-first",
      }),
    );
  });

  it("passes period + the switcher's ownerId when a period is provided", () => {
    mockUseQuery.mockReturnValue(createLoadingQueryResult());

    renderHook(() => useUsage("2026-06"));

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: { period: "2026-06", ownerId: "tea-1" },
      }),
    );
  });

  it("passes only ownerId when no period is provided", () => {
    mockUseQuery.mockReturnValue(createLoadingQueryResult());

    renderHook(() => useUsage());

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ variables: { ownerId: "tea-1" } }),
    );
  });

  it("exposes the period field from the response summary", () => {
    mockUseQuery.mockReturnValue(
      createSuccessQueryResult({
        usage: {
          workspaceId: "ws-abc",
          period: "2026-06",
        },
      }),
    );

    const { result } = renderHook(() => useUsage("2026-06"));

    expect(result.current.summary?.period).toBe("2026-06");
  });

  it("returns empty-string period when the field is absent from the response", () => {
    mockUseQuery.mockReturnValue(
      createSuccessQueryResult({
        usage: {
          workspaceId: "ws-abc",
        },
      }),
    );

    const { result } = renderHook(() => useUsage());

    expect(result.current.summary?.period).toBe("");
  });

  it("skips the query and reports loading until the switcher's selection resolves", () => {
    currentWorkspaceId = null;
    mockUseQuery.mockReturnValue(createLoadingQueryResult());

    const { result } = renderHook(() => useUsage());

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ skip: true }),
    );
    expect(result.current.loading).toBe(true);
  });
});

import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const gate = vi.hoisted(() => ({ runs: 0 }));
vi.mock("@/features/usage/context/payment-required-context", () => ({
  usePaymentRequiredGate: () => ({
    run: <T>(action: () => Promise<T>) => {
      gate.runs++;
      return action();
    },
  }),
}));

import { usePostgresAdminApps } from "@/features/databases/hooks/use-postgres-admin-apps";

const refetch = vi.fn();
const startPolling = vi.fn();
const stopPolling = vi.fn();

function serve(data: unknown) {
  mockUseQuery.mockReturnValue({
    data,
    loading: false,
    error: undefined,
    refetch,
    startPolling,
    stopPolling,
  });
}

const page = (phase: string | null) => ({
  database: { id: "dpg-1", projectId: "prj-1", environmentId: "env-1" },
  postgresAdminApps: [
    {
      id: "pgadmin",
      name: "pgAdmin",
      plan: "starter",
      diskSizeGB: 1,
      monthlyUsd: "4.90",
      diskMonthlyUsd: "0.175",
      service: null,
    },
    {
      id: "pghero",
      name: "PgHero",
      plan: "free",
      diskSizeGB: null,
      monthlyUsd: "0.00",
      diskMonthlyUsd: null,
      service:
        phase === null
          ? null
          : {
              id: "srv-1",
              name: "PgHero-dpg-1",
              phase,
              suspended: "not_suspended",
              type: "web_service",
            },
    },
    // An app this dashboard does not know is not drawn.
    { id: "adminer", name: "Adminer", plan: "free", service: null },
  ],
});

beforeEach(() => {
  mockUseQuery.mockReset();
  mockUseMutation.mockReset();
  refetch.mockReset().mockResolvedValue({});
  startPolling.mockReset();
  stopPolling.mockReset();
  gate.runs = 0;
});

describe("usePostgresAdminApps", () => {
  it("maps the catalog, the deployed service and the database's placement", () => {
    serve(page("Running"));
    mockUseMutation.mockReturnValue([vi.fn()]);
    const { result } = renderHook(() => usePostgresAdminApps("dpg-1"));
    expect(result.current.apps.map((a) => a.id)).toEqual(["pgadmin", "pghero"]);
    expect(result.current.apps[0]).toMatchObject({
      monthlyUsd: "4.90",
      diskMonthlyUsd: "0.175",
      service: null,
    });
    expect(result.current.apps[1].service).toMatchObject({
      id: "srv-1",
      phase: "Running",
      suspended: false,
    });
    expect(result.current).toMatchObject({
      projectId: "prj-1",
      environmentId: "env-1",
    });
  });

  it("polls fast while a deployed app's service builds, at the baseline once it runs", () => {
    serve(page("Building"));
    mockUseMutation.mockReturnValue([vi.fn()]);
    const { rerender } = renderHook(() => usePostgresAdminApps("dpg-1"));
    expect(startPolling).toHaveBeenLastCalledWith(3_000);
    serve(page("Running"));
    rerender();
    expect(startPolling).toHaveBeenLastCalledWith(25_000);
  });

  it("deploys through the payment gate with the dialog's login, then re-reads the page", async () => {
    serve(page(null));
    const mutate = vi.fn().mockResolvedValue({
      data: { deployPostgresAdminApp: { id: "srv-2", name: "PgHero-dpg-1" } },
    });
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => usePostgresAdminApps("dpg-1"));
    let created: { id: string; name: string } | undefined;
    await act(async () => {
      created = await result.current.deploy("pghero", {
        username: "hero",
        password: "pw",
      });
    });
    expect(gate.runs).toBe(1);
    expect(mutate).toHaveBeenCalledWith({
      variables: {
        databaseId: "dpg-1",
        app: "pghero",
        username: "hero",
        password: "pw",
      },
    });
    expect(refetch).toHaveBeenCalled();
    expect(created).toEqual({ id: "srv-2", name: "PgHero-dpg-1" });
  });

  it("rejects a deploy that answers no service", async () => {
    serve(page(null));
    mockUseMutation.mockReturnValue([
      vi.fn().mockResolvedValue({ data: { deployPostgresAdminApp: null } }),
    ]);
    const { result } = renderHook(() => usePostgresAdminApps("dpg-1"));
    await expect(
      result.current.deploy("pghero", { username: "u", password: "p" }),
    ).rejects.toThrow();
  });
});

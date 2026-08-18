import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import type { ServiceView } from "@/features/services/types";

// --- mocks ---------------------------------------------------------------

const mutateCalls: { op: string; id: string; confirm?: string }[] = [];
let rejectNext: Error | null = null;

vi.mock("@apollo/client/react", () => ({
  // Each useMutation(doc) returns a fn tagged with the operation name so the
  // test can assert which Render-named mutation fired.
  // `id` field in mutateCalls holds whichever of variables.id or
  // variables.serviceId is present (suspend/resume use `id`; triggerDeploy
  // uses `serviceId` after the w2/m30 restart consolidation).
  useMutation: (doc: { definitions: { name: { value: string } }[] }) => {
    const op = doc.definitions[0].name.value;
    const fn = vi.fn(
      async ({
        variables,
      }: {
        variables: { id?: string; serviceId?: string; confirm?: string };
      }) => {
        mutateCalls.push({
          op,
          id: variables.id ?? variables.serviceId ?? "",
          confirm: variables.confirm,
        });
        if (rejectNext) {
          const err = rejectNext;
          rejectNext = null;
          throw err;
        }
        return { data: {} };
      },
    );
    return [fn, {}];
  },
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (m: string) => toastSuccess(m),
    error: (m: string) => toastError(m),
  },
}));

// --- helpers -------------------------------------------------------------

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    slug: null,
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: null,
    internalAddress: null,
    createdAt: null,
    sshAddress: null,
    replicas: 1,
    revision: "r1",
    plan: null,
    idleTTLSeconds: null,
    schedule: null,
    command: null,
    runs: [],
    repo: null,
    branch: null,
    rootDir: null,
    runtime: null,
    builder: null,
    buildCommand: null,
    startCommand: null,
    dockerfilePath: null,
    registryCredentialId: null,
    buildFilter: null,
    autoDeploy: null,
    notifyOnFail: null,
    notificationsToSend: null,
    healthCheckPath: null,
    maxShutdownDelaySeconds: null,
    preDeployCommand: null,
    renderSubdomainPolicy: null,
    publishPath: null,
    routes: [],
    headers: [],
    ipAllowList: null,
    ipAllowListEntries: null,
    maintenanceMode: null,
    ...overrides,
  };
}

beforeEach(() => {
  mutateCalls.length = 0;
  rejectNext = null;
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useServiceLifecycle", () => {
  it("fires the matching Render mutation for each verb", async () => {
    const refetch = vi.fn(async () => [
      svc({ suspended: true, phase: "Hibernated" }),
    ]);
    const { result } = renderHook(() =>
      useServiceLifecycle({ refetch, pollIntervalMs: 0, maxPolls: 1 }),
    );

    await act(async () => {
      await result.current.run("suspend", svc());
    });
    await act(async () => {
      await result.current.run("resume", svc({ suspended: true }));
    });
    await act(async () => {
      await result.current.run("restart", svc());
    });

    expect(mutateCalls.map((c) => c.op)).toEqual([
      "SuspendService",
      "ResumeService",
      "TriggerDeploy", // restart consolidated into triggerDeploy (w2/m30)
    ]);
    expect(mutateCalls.every((c) => c.id === "app")).toBe(true);
  });

  it("polls the list until the row converges, then stops", async () => {
    // First poll still shows the old state, second shows converged.
    const refetch = vi
      .fn()
      .mockResolvedValueOnce([svc({ suspended: false })])
      .mockResolvedValueOnce([svc({ suspended: true, phase: "Hibernated" })])
      .mockResolvedValue([svc({ suspended: true, phase: "Hibernated" })]);

    const { result } = renderHook(() =>
      useServiceLifecycle({ refetch, pollIntervalMs: 0, maxPolls: 5 }),
    );

    await act(async () => {
      await result.current.run("suspend", svc());
    });

    // stopped as soon as it saw suspended=true (2 polls), not all 5
    expect(refetch).toHaveBeenCalledTimes(2);
    expect(toastSuccess).toHaveBeenCalledOnce();
  });

  it("clears pending and toasts an error when the mutation fails", async () => {
    rejectNext = new Error("boom");
    const refetch = vi.fn(async () => [svc()]);
    const { result } = renderHook(() =>
      useServiceLifecycle({ refetch, pollIntervalMs: 0, maxPolls: 1 }),
    );

    await act(async () => {
      await result.current.run("suspend", svc());
    });

    expect(toastError).toHaveBeenCalledOnce();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(result.current.pending).toBeNull();
    // a failed mutation does not poll for convergence
    expect(refetch).not.toHaveBeenCalled();
  });

  it("returns the exact protected phrase and sends it on the retry", async () => {
    rejectNext = new Error(
      'service is in a protected environment; retry with confirm="sudo suspend service app"',
    );
    const refetch = vi.fn(async () => [
      svc({ suspended: true, phase: "Hibernated" }),
    ]);
    const { result } = renderHook(() =>
      useServiceLifecycle({ refetch, pollIntervalMs: 0, maxPolls: 1 }),
    );

    let first;
    await act(async () => {
      first = await result.current.run("suspend", svc());
    });

    expect(first).toEqual({
      status: "confirmation_required",
      confirmation: "sudo suspend service app",
    });
    expect(toastError).not.toHaveBeenCalled();
    expect(refetch).not.toHaveBeenCalled();

    await act(async () => {
      await result.current.run("suspend", svc(), "sudo suspend service app");
    });

    expect(mutateCalls).toEqual([
      { op: "SuspendService", id: "app", confirm: undefined },
      {
        op: "SuspendService",
        id: "app",
        confirm: "sudo suspend service app",
      },
    ]);
    expect(toastSuccess).toHaveBeenCalledOnce();
    expect(refetch).toHaveBeenCalledOnce();
  });
});

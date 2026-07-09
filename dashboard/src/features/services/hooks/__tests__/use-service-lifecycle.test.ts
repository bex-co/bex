import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import type { ServiceView } from "@/features/services/types";

// --- mocks ---------------------------------------------------------------

const mutateCalls: { op: string; id: string }[] = [];
let rejectNext = false;

vi.mock("@apollo/client/react", () => ({
  // Each useMutation(doc) returns a fn tagged with the operation name so the
  // test can assert which Render-named mutation fired.
  useMutation: (doc: { definitions: { name: { value: string } }[] }) => {
    const op = doc.definitions[0].name.value;
    const fn = vi.fn(async ({ variables }: { variables: { id: string } }) => {
      mutateCalls.push({ op, id: variables.id });
      if (rejectNext) {
        rejectNext = false;
        throw new Error("boom");
      }
      return { data: {} };
    });
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
    suspended: false,
    phase: "Running",
    url: null,
    createdAt: null,
    replicas: 1,
    revision: "r1",
    ...overrides,
  };
}

beforeEach(() => {
  mutateCalls.length = 0;
  rejectNext = false;
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
      "RestartServer",
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
    rejectNext = true;
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
});

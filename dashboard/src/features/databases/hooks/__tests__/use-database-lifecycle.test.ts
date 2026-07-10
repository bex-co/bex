import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useDatabaseLifecycle } from "@/features/databases/hooks/use-database-lifecycle";
import type { DatabaseView } from "@/features/databases/types";

// --- mocks ---------------------------------------------------------------

const mutateCalls: { op: string; id: string }[] = [];
let rejectNext = false;

vi.mock("@apollo/client/react", () => ({
  // Each useMutation(doc) returns a fn tagged with the operation name so the
  // test can assert which mutation fired.
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

function db(overrides: Partial<DatabaseView> = {}): DatabaseView {
  return {
    id: "shop-db",
    name: "shop-db",
    status: "available",
    plan: "basic-1gb",
    version: "16",
    diskSizeGB: 5,
    createdAt: null,
    public: false,
    suspended: "not_suspended",
    ...overrides,
  };
}

beforeEach(() => {
  mutateCalls.length = 0;
  rejectNext = false;
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useDatabaseLifecycle", () => {
  it("fires the matching mutation and refetches for each verb", async () => {
    const refetch = vi.fn();
    const { result } = renderHook(() => useDatabaseLifecycle({ refetch }));

    await act(async () => {
      await result.current.run("suspend", db());
    });
    await act(async () => {
      await result.current.run("resume", db({ suspended: "suspended" }));
    });
    await act(async () => {
      await result.current.run("restart", db());
    });

    expect(mutateCalls.map((c) => c.op)).toEqual([
      "SuspendDatabase",
      "ResumeDatabase",
      "RestartDatabase",
    ]);
    expect(mutateCalls.every((c) => c.id === "shop-db")).toBe(true);
    expect(refetch).toHaveBeenCalledTimes(3);
    expect(toastSuccess).toHaveBeenCalledTimes(3);
    expect(toastError).not.toHaveBeenCalled();
  });

  it("surfaces an error toast (and still refetches nothing extra) on failure", async () => {
    const refetch = vi.fn();
    const { result } = renderHook(() => useDatabaseLifecycle({ refetch }));

    rejectNext = true;
    await act(async () => {
      await result.current.run("suspend", db());
    });

    expect(toastError).toHaveBeenCalledTimes(1);
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(refetch).not.toHaveBeenCalled();
    expect(result.current.pending).toBeNull();
  });
});

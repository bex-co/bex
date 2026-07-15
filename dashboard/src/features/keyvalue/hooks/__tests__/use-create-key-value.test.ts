import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...a: unknown[]) => toastSuccess(...a),
    error: (...a: unknown[]) => toastError(...a),
  },
}));

// The workspace a create lands in is the switcher's selection (WorkspaceProvider),
// never one the hook resolves itself — w6/m14.
let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

import { useCreateKeyValue } from "@/features/keyvalue/hooks/use-create-key-value";

const input = { name: "cache", plan: "starter", version: "8", public: false };

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  currentWorkspaceId = "tea-1";
});

describe("useCreateKeyValue", () => {
  it("sends the switcher's workspace as ownerId", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { createKeyValue: { id: "kv-1" } } });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateKeyValue());
    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.create(input);
    });

    expect(id).toBe("kv-1");
    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: {
          name: "cache",
          ownerId: "tea-1",
          plan: "starter",
          version: "8",
          public: false,
          environmentId: undefined,
        },
      }),
    );
    expect(toastSuccess).toHaveBeenCalledWith("Creating cache…");
  });

  it("forwards an optional environment assignment", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { createKeyValue: { id: "kv-1" } } });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateKeyValue());
    await act(async () => {
      await result.current.create({ ...input, environmentId: "env-1" });
    });

    expect(mutate.mock.calls[0][0].variables.environmentId).toBe("env-1");
  });

  it("follows a workspace switch — the next create carries the new ownerId", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { createKeyValue: { id: "kv-2" } } });
    mockUseMutation.mockReturnValue([mutate]);

    const { result, rerender } = renderHook(() => useCreateKeyValue());
    currentWorkspaceId = "tea-2";
    rerender();

    await act(async () => {
      await result.current.create(input);
    });

    expect(mutate.mock.calls[0][0].variables.ownerId).toBe("tea-2");
  });

  it("refuses to create until the workspace selection resolves", async () => {
    const mutate = vi.fn();
    mockUseMutation.mockReturnValue([mutate]);
    currentWorkspaceId = null;

    const { result } = renderHook(() => useCreateKeyValue());
    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.create(input);
    });

    expect(mutate).not.toHaveBeenCalled();
    expect(id).toBeNull();
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't create cache. Please try again.",
    );
  });
});

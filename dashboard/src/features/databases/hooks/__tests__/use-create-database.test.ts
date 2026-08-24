import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";

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

import { useCreateDatabase } from "@/features/databases/hooks/use-create-database";

const input = {
  name: "db",
  databaseName: "orders_data",
  databaseUser: "orders_owner",
  plan: "starter",
  version: "16",
  diskSizeGB: 10,
  public: false,
};

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  currentWorkspaceId = "tea-1";
});

describe("useCreateDatabase", () => {
  it("sends the switcher's workspace as ownerId", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { createDatabase: { id: "dpg-1" } } });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateDatabase());
    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.create(input);
    });

    expect(id).toBe("dpg-1");
    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: {
          name: "db",
          databaseName: "orders_data",
          databaseUser: "orders_owner",
          ownerId: "tea-1",
          plan: "starter",
          version: "16",
          diskSizeGB: 10,
          public: false,
          environmentId: undefined,
        },
      }),
    );
    expect(toastSuccess).toHaveBeenCalledWith("Creating db…");
  });

  it("forwards an optional environment assignment", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { createDatabase: { id: "dpg-1" } } });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateDatabase());
    await act(async () => {
      await result.current.create({ ...input, environmentId: "env-1" });
    });

    expect(mutate.mock.calls[0][0].variables.environmentId).toBe("env-1");
  });

  it("follows a workspace switch — the next create carries the new ownerId", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { createDatabase: { id: "dpg-2" } } });
    mockUseMutation.mockReturnValue([mutate]);

    const { result, rerender } = renderHook(() => useCreateDatabase());
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

    const { result } = renderHook(() => useCreateDatabase());
    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.create(input);
    });

    expect(mutate).not.toHaveBeenCalled();
    expect(id).toBeNull();
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't create db. Please try again.",
    );
  });

  it("does not invent a name-based id when the create response omits its id", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { createDatabase: { id: null } } });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateDatabase());
    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.create(input);
    });

    expect(id).toBeNull();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't create db. Please try again.",
    );
  });

  // w6/m49: a duplicate-name conflict shows the backend's specific reason
  // instead of the generic "Please try again" toast.
  it("shows the backend's specific reason on a name conflict", async () => {
    const mutate = vi.fn().mockRejectedValue(
      new CombinedGraphQLErrors({
        data: null,
        errors: [
          {
            message: 'a Postgres database named "db" already exists in this workspace',
            extensions: { code: "CONFLICT" },
          },
        ],
      }),
    );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateDatabase());
    await act(async () => {
      await result.current.create(input);
    });

    expect(toastError).toHaveBeenCalledWith(
      'A Postgres database named "db" already exists in this workspace',
    );
  });

  // The conflict branch must be additive: a non-conflict, non-cap-limit error
  // still falls through to the generic toast unchanged.
  it("falls through to the generic toast for a non-conflict error", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("network error"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateDatabase());
    await act(async () => {
      await result.current.create(input);
    });

    expect(toastError).toHaveBeenCalledWith(
      "Couldn't create db. Please try again.",
    );
  });
});

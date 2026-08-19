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

import { useCreateWorkspace } from "@/features/workspaces/hooks/use-create-workspace";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useCreateWorkspace", () => {
  it("resolves the created workspace and toasts success", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: {
        createWorkspace: {
          id: "tea-1",
          name: "acme-staging",
          plan: "hobby",
          role: "admin",
          createdAt: "2026-07-09T00:00:00Z",
        },
      },
    });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateWorkspace());
    let workspace;
    await act(async () => {
      workspace = await result.current.create("acme-staging", "hobby");
    });

    expect(workspace).toEqual({
      id: "tea-1",
      name: "acme-staging",
      plan: "hobby",
      role: "admin",
      createdAt: "2026-07-09T00:00:00Z",
    });
    expect(mutate).toHaveBeenCalledWith({
      variables: { name: "acme-staging", plan: "hobby" },
      update: expect.any(Function),
    });
    expect(toastSuccess).toHaveBeenCalledWith("Created acme-staging");
    expect(result.current.error).toBeNull();
  });

  it("adds the created workspace to the shared list cache before returning", async () => {
    const created = {
      __typename: "Workspace",
      id: "tea-2",
      name: "acme-staging",
      plan: "hobby",
      role: "admin",
      createdAt: null,
    };
    const mutate = vi.fn().mockResolvedValue({
      data: { createWorkspace: created },
    });
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useCreateWorkspace());

    await act(() => result.current.create("acme-staging", "hobby"));

    const update = mutate.mock.calls[0][0].update;
    let updated: unknown;
    const updateQuery = vi.fn(
      (_options: unknown, updater: (existing: unknown) => unknown) => {
        updated = updater({
          workspaces: [
            {
              __typename: "Workspace",
              id: "tea-1",
              name: "acme",
              plan: "hobby",
              role: "admin",
              createdAt: null,
            },
          ],
        });
      },
    );
    update(
      { updateQuery } as never,
      { data: { createWorkspace: created } } as never,
    );

    expect(updated).toEqual({
      workspaces: [
        expect.objectContaining({ id: "tea-1" }),
        expect.objectContaining({ id: "tea-2" }),
      ],
    });
  });

  it("surfaces the backend's plan-limit refusal inline (not just a toast) — the 6th Hobby workspace", async () => {
    const mutate = vi.fn().mockRejectedValue(
      new CombinedGraphQLErrors({
        errors: [
          { message: "bad request: at most 5 hobby workspaces per user" },
        ],
      } as never),
    );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateWorkspace());
    let workspace;
    await act(async () => {
      workspace = await result.current.create("acme-6th", "hobby");
    });

    expect(workspace).toBeNull();
    expect(result.current.error).toBe(
      "bad request: at most 5 hobby workspaces per user",
    );
  });

  it("falls back to a generic error message for a non-GraphQL failure", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("network down"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateWorkspace());
    await act(async () => {
      await result.current.create("acme-staging", "hobby");
    });

    expect(result.current.error).toBe("network down");
  });

  it("clears a previous error on a fresh attempt", async () => {
    const mutate = vi
      .fn()
      .mockRejectedValueOnce(new Error("first failure"))
      .mockResolvedValueOnce({
        data: {
          createWorkspace: {
            id: "tea-2",
            name: "acme-2",
            plan: "hobby",
            role: "admin",
            createdAt: null,
          },
        },
      });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateWorkspace());
    await act(async () => {
      await result.current.create("acme-2", "hobby");
    });
    expect(result.current.error).toBe("first failure");

    await act(async () => {
      await result.current.create("acme-2", "hobby");
    });
    expect(result.current.error).toBeNull();
  });
});

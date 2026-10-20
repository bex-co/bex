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

import i18n from "@/i18n/init";
import { uncodedGraphQLError } from "@/test/mocks/apollo";
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
    const mutate = vi
      .fn()
      .mockRejectedValue(
        uncodedGraphQLError("bad request: at most 5 hobby workspaces per user"),
      );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateWorkspace());
    let workspace;
    await act(async () => {
      workspace = await result.current.create("acme-6th", "hobby");
    });

    expect(workspace).toBeNull();
    expect(result.current.error).toBe("At most 5 hobby workspaces per user");
  });

  // The browser's "Failed to fetch" is untranslated and names nothing the
  // user can act on, so a transport failure shows the dialog's own copy in
  // the user's language (w5/164).
  it("falls back to the translated generic message for a non-GraphQL failure", async () => {
    const mutate = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateWorkspace());
    await act(async () => {
      await result.current.create("acme-staging", "hobby");
    });
    expect(result.current.error).toBe("Couldn't create the workspace");

    await i18n.changeLanguage("zh");
    try {
      const zh = renderHook(() => useCreateWorkspace());
      await act(async () => {
        await zh.result.current.create("acme-staging", "hobby");
      });
      expect(zh.result.current.error).toBe("无法创建工作区");
    } finally {
      await i18n.changeLanguage("en");
    }
  });

  it("clears a previous error on a fresh attempt", async () => {
    const mutate = vi
      .fn()
      .mockRejectedValueOnce(new TypeError("Failed to fetch"))
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
    expect(result.current.error).toBe("Couldn't create the workspace");

    await act(async () => {
      await result.current.create("acme-2", "hobby");
    });
    expect(result.current.error).toBeNull();
  });
});

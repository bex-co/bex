import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate],
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-owner" }),
}));

import { useCreateProject } from "@/features/projects/hooks/use-create-project";

beforeEach(() => {
  mutate.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useCreateProject", () => {
  it("returns the immutable id supplied by the create response", async () => {
    mutate.mockResolvedValue({
      data: { createProject: { id: "prj-returned" } },
    });
    const { result } = renderHook(() => useCreateProject());

    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.create("friendly-name");
    });

    expect(id).toBe("prj-returned");
    expect(mutate).toHaveBeenCalledWith({
      variables: { name: "friendly-name", ownerId: "tea-owner" },
    });
    expect(toastSuccess).toHaveBeenCalled();
  });

  it("treats a response without an id as failure", async () => {
    mutate.mockResolvedValue({ data: { createProject: { id: null } } });
    const { result } = renderHook(() => useCreateProject());

    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.create("friendly-name");
    });

    expect(id).toBeNull();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalled();
  });
});

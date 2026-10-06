import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

import { useRenameKeyValue } from "@/features/keyvalue/hooks/use-rename-key-value";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useRenameKeyValue", () => {
  it("renames through the stable key value id", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: { renameKeyValue: { id: "red-stable", name: "new-name" } },
    });
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useRenameKeyValue());

    let ok = false;
    await act(async () => {
      ok = await result.current.rename("red-stable", "new-name");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "red-stable", name: "new-name" },
    });
    expect(toastSuccess).toHaveBeenCalledWith(
      "Renamed Key Value store to new-name.",
    );
  });

  it("surfaces a workspace name collision", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(new Error("already exists in this workspace")),
    ]);
    const { result } = renderHook(() => useRenameKeyValue());

    await act(async () => {
      await result.current.rename("red-stable", "taken");
    });

    expect(toastError).toHaveBeenCalledWith(
      "A Key Value store with that name already exists in this workspace.",
    );
  });

  it("words an invalid name by its code, not the server's wording", async () => {
    const refused = (extensions: Record<string, unknown>) =>
      new CombinedGraphQLErrors({
        data: null,
        errors: [{ message: "name must use lowercase letters", extensions }],
      });
    mockUseMutation.mockReturnValue([
      vi
        .fn()
        .mockRejectedValueOnce(
          refused({
            code: "RESOURCE_NAME_INVALID",
            field: "name",
            maxLength: 30,
          }),
        )
        .mockRejectedValueOnce(refused({ code: "BAD_REQUEST" })),
    ]);
    const { result } = renderHook(() => useRenameKeyValue());

    await act(async () => {
      await result.current.rename("red-stable", "Bad_Name");
    });
    expect(toastError).toHaveBeenLastCalledWith(
      "Use lowercase letters, digits, and hyphens (up to 30 characters); don't start or end with a hyphen.",
    );

    await act(async () => {
      await result.current.rename("red-stable", "Bad_Name");
    });
    expect(toastError).toHaveBeenLastCalledWith(
      "Name must use lowercase letters",
    );
  });
});

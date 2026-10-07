import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";
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

import { useRenameDatabase } from "@/features/databases/hooks/use-rename-database";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useRenameDatabase", () => {
  it("renames through the stable database id", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: { renameDatabase: { id: "dpg-stable", name: "new-name" } },
    });
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useRenameDatabase());

    let ok = false;
    await act(async () => {
      ok = await result.current.rename("dpg-stable", "new-name");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "dpg-stable", name: "new-name" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Renamed database to new-name.");
  });

  it("surfaces a workspace name collision", async () => {
    mockUseMutation.mockReturnValue([
      vi
        .fn()
        .mockRejectedValue(
          codedGraphQLError("CONFLICT", {}, "already exists in this workspace"),
        ),
    ]);
    const { result } = renderHook(() => useRenameDatabase());

    await act(async () => {
      await result.current.rename("dpg-stable", "taken");
    });

    expect(toastError).toHaveBeenCalledWith(
      "A database with that name already exists in this workspace.",
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
    const { result } = renderHook(() => useRenameDatabase());

    await act(async () => {
      await result.current.rename("dpg-stable", "Bad_Name");
    });
    expect(toastError).toHaveBeenLastCalledWith(
      "Use lowercase letters, digits, and hyphens (up to 30 characters); don't start or end with a hyphen.",
    );

    await act(async () => {
      await result.current.rename("dpg-stable", "Bad_Name");
    });
    expect(toastError).toHaveBeenLastCalledWith(
      "Name must use lowercase letters",
    );
  });
});

// A taken name is recognized by bex-api's CONFLICT code (w6/m49), not by
// "already exists" in its wording (w5/m130).
describe("useRenameDatabase without a code", () => {
  it("does not read a name collision from the wording alone", async () => {
    mockUseMutation.mockReturnValue([
      vi
        .fn()
        .mockRejectedValue(
          uncodedGraphQLError(
            `a Postgres database named "taken" already exists in this workspace`,
          ),
        ),
    ]);
    const { result } = renderHook(() => useRenameDatabase());
    await act(async () => {
      await result.current.rename("dpg-stable", "taken");
    });
    expect(toastError).toHaveBeenCalledTimes(1);
    expect(toastError).not.toHaveBeenCalledWith(
      "A database with that name already exists in this workspace.",
    );
  });
});

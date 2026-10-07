import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

import { useChangeRole } from "@/features/team/hooks/use-change-role";
import { useRemoveMember } from "@/features/team/hooks/use-remove-member";

const LAST_ADMIN = "A workspace must keep at least one admin.";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastError.mockReset();
});

// The last-admin refusal is recognized by its code, LAST_ADMIN, not by "last
// admin" in its wording (w5/m130).
describe.each([
  [
    "removal",
    async () => {
      const { result } = renderHook(() => useRemoveMember("tea-1"));
      await act(async () => {
        await result.current.removeMember("admin-1");
      });
    },
  ],
  [
    "demotion",
    async () => {
      const { result } = renderHook(() => useChangeRole("tea-1"));
      await act(async () => {
        await result.current.changeRole("admin-1", "VIEWER");
      });
    },
  ],
])("the last admin's %s", (_verb, attempt) => {
  it("is refused with the last-admin copy when bex-api codes it", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(codedGraphQLError("LAST_ADMIN")),
    ]);
    await attempt();
    expect(toastError).toHaveBeenCalledWith(LAST_ADMIN);
  });

  it("is not recognized from the wording alone", async () => {
    mockUseMutation.mockReturnValue([
      vi
        .fn()
        .mockRejectedValue(
          uncodedGraphQLError(
            "cannot remove or demote the last admin of a workspace",
          ),
        ),
    ]);
    await attempt();
    expect(toastError).toHaveBeenCalledTimes(1);
    expect(toastError).not.toHaveBeenCalledWith(LAST_ADMIN);
  });
});

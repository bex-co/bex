import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";

const mutate = vi.fn();
const mockUseMutation = vi.fn(() => [mutate]);
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

import { useSetEnvironmentACL } from "@/features/environments/hooks/use-set-environment-acl";

beforeEach(() => {
  mutate.mockReset();
  mockUseMutation.mockClear();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useSetEnvironmentACL", () => {
  it("full-replaces all ACL fields and refreshes environments", async () => {
    mutate.mockResolvedValue({ data: {} });
    const { result } = renderHook(() => useSetEnvironmentACL());

    let ok;
    await act(async () => {
      ok = await result.current.saveACL("env-1", "production", {
        protectedStatus: "protected",
        networkIsolationEnabled: true,
        ipAllowListEntries: [
          { cidrBlock: "10.0.0.0/8", description: "office" },
        ],
      });
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: {
        id: "env-1",
        protectedStatus: "protected",
        networkIsolationEnabled: true,
        ipAllowListEntries: [
          { cidrBlock: "10.0.0.0/8", description: "office" },
        ],
      },
    });
    expect(mockUseMutation.mock.calls[0][1]).toMatchObject({
      refetchQueries: ["Environments"],
      awaitRefetchQueries: true,
    });
    expect(ok).toBe(true);
    expect(toastSuccess).toHaveBeenCalledWith(
      'Settings for "production" saved.',
    );
  });

  it("keeps the editor open by returning false and toasts on failure", async () => {
    mutate.mockRejectedValue(new Error("bad CIDR"));
    const { result } = renderHook(() => useSetEnvironmentACL());

    let ok;
    await act(async () => {
      ok = await result.current.saveACL("env-1", "production", {
        protectedStatus: "unprotected",
        networkIsolationEnabled: false,
        ipAllowListEntries: [],
      });
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith(
      'Failed to save settings for "production".',
    );
  });
});

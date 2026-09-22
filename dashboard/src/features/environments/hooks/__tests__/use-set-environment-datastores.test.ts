import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";

const mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({ useMutation: () => [mutate] }));
const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

import { useSetEnvironmentDatabases } from "@/features/environments/hooks/use-set-environment-databases";
import { useSetEnvironmentKeyValues } from "@/features/environments/hooks/use-set-environment-keyvalues";

beforeEach(() => {
  vi.clearAllMocks();
});

describe.each([
  {
    kind: "database",
    useMembership: () => {
      const { setDatabases, busyId } = useSetEnvironmentDatabases();
      return { save: setDatabases, busyId };
    },
  },
  {
    kind: "key value",
    useMembership: () => {
      const { setKeyValues, busyId } = useSetEnvironmentKeyValues();
      return { save: setKeyValues, busyId };
    },
  },
])("$kind membership", ({ useMembership }) => {
  it("surfaces a server refusal and releases the form for retry", async () => {
    let rejectMutation!: (error: unknown) => void;
    mutate.mockImplementationOnce(
      () =>
        new Promise((_, reject) => {
          rejectMutation = reject;
        }),
    );
    const { result } = renderHook(useMembership);
    let pending!: Promise<boolean>;
    act(() => {
      pending = result.current.save("env-1", "production", ["missing"]);
    });
    expect(result.current.busyId).toBe("env-1");

    await act(async () => {
      rejectMutation(
        new CombinedGraphQLErrors({
          errors: [
            {
              message:
                'forbidden: "missing" does not belong to workspace "tea-1"',
            },
          ],
        }),
      );
      expect(await pending).toBe(false);
    });
    expect(result.current.busyId).toBeNull();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalledWith(
      'Forbidden: "missing" does not belong to workspace "tea-1"',
    );
  });
});

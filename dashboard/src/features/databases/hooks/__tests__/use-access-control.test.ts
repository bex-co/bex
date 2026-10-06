import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";

const createUserMut = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({}),
  useQuery: () => ({ data: undefined, refetch: vi.fn() }),
  useMutation: () => [createUserMut, { loading: false }],
}));

const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: (...a: unknown[]) => toastError(...a) },
}));

import { useAccessControl } from "@/features/databases/hooks/use-access-control";

beforeEach(() => {
  createUserMut.mockReset();
  toastError.mockReset();
});

describe("useAccessControl createUser", () => {
  // w5/m118: add-user refusals are coded; a role PostgreSQL owns is worded
  // locally from its code rather than echoing the server's English.
  it("words a reserved role from POSTGRES_IDENTIFIER_RESERVED", async () => {
    createUserMut.mockRejectedValue(
      new CombinedGraphQLErrors({
        data: null,
        errors: [
          {
            message:
              'name "pg_monitor" is reserved by PostgreSQL; choose another name',
            extensions: { code: "POSTGRES_IDENTIFIER_RESERVED", field: "name" },
          },
        ],
      }),
    );
    const { result } = renderHook(() => useAccessControl("dpg-1"));
    let password: string | null | undefined;
    await act(async () => {
      password = await result.current.createUser("pg_monitor");
    });
    expect(password).toBeNull();
    expect(toastError).toHaveBeenCalledWith(
      "“pg_monitor” is reserved by PostgreSQL. Choose another name.",
    );
  });

  it("keeps the server's reason for any other refusal", async () => {
    createUserMut.mockRejectedValue(
      new Error('user "analytics" already exists'),
    );
    const { result } = renderHook(() => useAccessControl("dpg-1"));
    await act(async () => {
      await result.current.createUser("analytics");
    });
    expect(toastError).toHaveBeenCalledWith(
      `Couldn't create the user: user "analytics" already exists`,
    );
  });
});

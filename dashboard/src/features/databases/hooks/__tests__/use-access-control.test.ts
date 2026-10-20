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

import i18n from "@/i18n/init";
import { uncodedGraphQLError } from "@/test/mocks/apollo";
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
      uncodedGraphQLError('bad request: user "analytics" already exists'),
    );
    const { result } = renderHook(() => useAccessControl("dpg-1"));
    await act(async () => {
      await result.current.createUser("analytics");
    });
    expect(toastError).toHaveBeenCalledWith('User "analytics" already exists');
  });

  // w5/166: the browser's "Failed to fetch" is untranslated and names
  // nothing the user can act on, so the toast reads the copy alone.
  it("shows reason-less copy for a transport failure, in the user's language", async () => {
    createUserMut.mockRejectedValue(new TypeError("Failed to fetch"));
    const { result } = renderHook(() => useAccessControl("dpg-1"));
    await act(async () => {
      await result.current.createUser("analytics");
    });
    expect(toastError).toHaveBeenLastCalledWith(
      "Couldn't create the user. Please try again.",
    );

    await i18n.changeLanguage("zh");
    try {
      const zh = renderHook(() => useAccessControl("dpg-1"));
      await act(async () => {
        await zh.result.current.createUser("analytics");
      });
      expect(toastError).toHaveBeenLastCalledWith("无法创建用户，请重试。");
    } finally {
      await i18n.changeLanguage("en");
    }
  });
});

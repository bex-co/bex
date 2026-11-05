import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";

const createCredentialMut = vi.fn();
const writeQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ cache: { writeQuery } }),
  useQuery: () => ({ data: undefined, refetch: vi.fn() }),
  useMutation: () => [createCredentialMut, { loading: false }],
}));

const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: (...a: unknown[]) => toastError(...a) },
}));

import i18n from "@/i18n/init";
import { uncodedGraphQLError } from "@/test/mocks/apollo";
import { useAccessControl } from "@/features/databases/hooks/use-access-control";

beforeEach(() => {
  createCredentialMut.mockReset();
  toastError.mockReset();
});

describe("useAccessControl createCredential", () => {
  // w5/m118: add-credential refusals are coded; a role PostgreSQL owns is worded
  // locally from its code rather than echoing the server's English.
  it("words a reserved role from POSTGRES_IDENTIFIER_RESERVED", async () => {
    createCredentialMut.mockRejectedValue(
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
    let created: boolean | undefined;
    await act(async () => {
      created = await result.current.createCredential("pg_monitor");
    });
    expect(created).toBe(false);
    expect(toastError).toHaveBeenCalledWith(
      "“pg_monitor” is reserved by PostgreSQL. Choose another name.",
    );
  });

  // w5/m265: Render's create sends `username` and echoes no password.
  it("sends Render's username and stores the answered list", async () => {
    const list = [
      { __typename: "DatabaseCredential", username: "rotated", default: true },
    ];
    createCredentialMut.mockResolvedValue({
      data: { createDatabaseCredential: list },
    });
    const { result } = renderHook(() => useAccessControl("dpg-1"));
    let created: boolean | undefined;
    await act(async () => {
      created = await result.current.createCredential("rotated");
    });
    expect(created).toBe(true);
    expect(createCredentialMut).toHaveBeenCalledWith({
      variables: { id: "dpg-1", username: "rotated" },
    });
    expect(writeQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: { id: "dpg-1" },
        data: { databaseCredentials: list },
      }),
    );
  });

  it("keeps the server's reason for any other refusal", async () => {
    createCredentialMut.mockRejectedValue(
      uncodedGraphQLError('bad request: user "analytics" already exists'),
    );
    const { result } = renderHook(() => useAccessControl("dpg-1"));
    await act(async () => {
      await result.current.createCredential("analytics");
    });
    expect(toastError).toHaveBeenCalledWith('User "analytics" already exists');
  });

  // w5/166: the browser's "Failed to fetch" is untranslated and names
  // nothing the user can act on, so the toast reads the copy alone.
  it("shows reason-less copy for a transport failure, in the user's language", async () => {
    createCredentialMut.mockRejectedValue(new TypeError("Failed to fetch"));
    const { result } = renderHook(() => useAccessControl("dpg-1"));
    await act(async () => {
      await result.current.createCredential("analytics");
    });
    expect(toastError).toHaveBeenLastCalledWith(
      "Couldn't create the credential. Please try again.",
    );

    await i18n.changeLanguage("zh");
    try {
      const zh = renderHook(() => useAccessControl("dpg-1"));
      await act(async () => {
        await zh.result.current.createCredential("analytics");
      });
      expect(toastError).toHaveBeenLastCalledWith("无法创建凭据，请重试。");
    } finally {
      await i18n.changeLanguage("en");
    }
  });
});

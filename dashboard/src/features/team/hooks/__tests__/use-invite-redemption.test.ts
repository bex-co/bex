import { CombinedGraphQLErrors, ServerError } from "@apollo/client/errors";
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { INVITE_TOKEN_STORAGE_KEY } from "@/common/lib/invite-token";

const TOKEN = "0123456789abcdef0123456789abcdef";
const mockUseMutation = vi.fn();
const mockRefetch = vi.fn();
const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));
vi.mock("@/common/hooks/use-translations", () => ({
  useTranslations: () => ({
    t: (key: string) => key,
  }),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ refetch: mockRefetch }),
}));

import { useInviteRedemption } from "../use-invite-redemption";

function gqlError(code: string) {
  return new CombinedGraphQLErrors({
    data: null,
    errors: [
      { message: "copy is not part of the contract", extensions: { code } },
    ],
  });
}

function serverError(statusCode: number) {
  return new ServerError(`status ${statusCode}`, {
    response: new Response(null, { status: statusCode }),
    bodyText: "",
  });
}

describe("useInviteRedemption", () => {
  beforeEach(() => {
    mockUseMutation.mockReset();
    mockRefetch.mockReset().mockResolvedValue(undefined);
    toastSuccess.mockReset();
    toastError.mockReset();
    window.sessionStorage.clear();
    window.history.replaceState(null, "", "/");
    window.sessionStorage.setItem(INVITE_TOKEN_STORAGE_KEY, TOKEN);
  });

  it("does not redeem on mount — navigation alone creates no membership", async () => {
    const mutate = vi.fn();
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useInviteRedemption());

    await waitFor(() => expect(result.current.pendingToken).toBe(TOKEN));
    expect(mutate).not.toHaveBeenCalled();
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBe(TOKEN);
  });

  it("decline clears the pending capability without calling accept", async () => {
    const mutate = vi.fn();
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useInviteRedemption());
    await waitFor(() => expect(result.current.pendingToken).toBe(TOKEN));

    act(() => {
      result.current.decline();
    });

    expect(result.current.pendingToken).toBeNull();
    expect(mutate).not.toHaveBeenCalled();
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBeNull();
  });

  it.each([
    ["network failure", new TypeError("network failed")],
    ["raw HTTP 404", serverError(404)],
    ["raw HTTP 409", serverError(409)],
  ])("retains the capability after ambiguous %s", async (_label, error) => {
    const mutate = vi.fn().mockRejectedValue(error);
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useInviteRedemption());
    await waitFor(() => expect(result.current.pendingToken).toBe(TOKEN));

    await act(async () => {
      await result.current.accept();
    });

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    expect(mutate).toHaveBeenCalledWith({ variables: { token: TOKEN } });
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBe(TOKEN);
    expect(toastError).toHaveBeenCalledWith("team.inviteAcceptError");
  });

  it("clears a stable terminal outcome using its GraphQL code", async () => {
    const mutate = vi.fn().mockRejectedValue(gqlError("INVITE_EXPIRED"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useInviteRedemption());
    await waitFor(() => expect(result.current.pendingToken).toBe(TOKEN));

    await act(async () => {
      await result.current.accept();
    });

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBeNull();
    expect(toastError).toHaveBeenCalledWith("team.inviteAcceptExpired");
  });

  it("does not restore a spent capability when refresh fails after commit", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: {
        acceptWorkspaceInvite: {
          workspaceId: "tea-joined",
          workspaceName: "Joined",
        },
      },
    });
    mockUseMutation.mockReturnValue([mutate]);
    mockRefetch.mockRejectedValue(new Error("refresh failed"));

    const { result } = renderHook(() => useInviteRedemption());
    await waitFor(() => expect(result.current.pendingToken).toBe(TOKEN));

    await act(async () => {
      await result.current.accept();
    });

    await waitFor(() => expect(toastSuccess).toHaveBeenCalled());
    await waitFor(() => expect(mockRefetch).toHaveBeenCalled());
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBeNull();
    expect(toastError).not.toHaveBeenCalled();
  });
});

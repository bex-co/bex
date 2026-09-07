import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { INVITE_TOKEN_STORAGE_KEY } from "@/common/lib/invite-token";

const mocks = vi.hoisted(() => ({
  mutate: vi.fn(),
  query: vi.fn(),
  select: vi.fn(),
  previewRefetch: vi.fn(),
  workspaces: [{ id: "tea-target" }],
  preview: {
    data: {
      workspaceInvitePreview: {
        workspaceId: "tea-target",
        workspaceName: "Acme",
        role: "DEVELOPER",
        alreadyMember: false,
      },
    },
    loading: false,
    error: undefined,
  },
}));
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mocks.mutate],
  useQuery: () => ({ ...mocks.preview, refetch: mocks.previewRefetch }),
  useApolloClient: () => ({ query: mocks.query }),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    workspaces: mocks.workspaces,
    setCurrentWorkspaceId: mocks.select,
  }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn() } }));
import { useInviteRedemption } from "../use-invite-redemption";
const TOKEN = "0123456789abcdef0123456789abcdef";

beforeEach(() => {
  vi.clearAllMocks();
  mocks.workspaces = [{ id: "tea-target" }];
  mocks.preview.data.workspaceInvitePreview.alreadyMember = false;
  mocks.mutate.mockResolvedValue({
    data: { acceptWorkspaceInvite: { workspaceId: "tea-target" } },
  });
  mocks.query.mockResolvedValue({
    data: {
      workspaces: [{ id: "tea-target" }],
      viewerCapabilities: { canView: true },
    },
  });
  window.history.replaceState(null, "", "/invite");
  window.sessionStorage.clear();
  window.sessionStorage.setItem(INVITE_TOKEN_STORAGE_KEY, TOKEN);
});

describe("invitation redemption", () => {
  it("never accepts or changes selection on mount", () => {
    renderHook(() => useInviteRedemption(TOKEN, vi.fn()));
    expect(mocks.mutate).not.toHaveBeenCalled();
    expect(mocks.select).not.toHaveBeenCalled();
  });
  it("selects the joined workspace only after access is confirmed", async () => {
    const opened = vi.fn();
    const { result } = renderHook(() => useInviteRedemption(TOKEN, opened));
    await act(() => result.current.accept());
    expect(mocks.mutate).toHaveBeenCalledWith({ variables: { token: TOKEN } });
    expect(mocks.select).toHaveBeenCalledWith("tea-target");
    expect(opened).toHaveBeenCalledOnce();
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBeNull();
  });
  it("waits for the workspace provider to observe a new membership before selecting it", async () => {
    mocks.workspaces = [{ id: "tea-personal" }];
    const opened = vi.fn();
    const { result, rerender } = renderHook(() =>
      useInviteRedemption(TOKEN, opened),
    );
    await act(() => result.current.accept());
    expect(mocks.select).not.toHaveBeenCalled();
    expect(opened).not.toHaveBeenCalled();
    mocks.workspaces = [{ id: "tea-personal" }, { id: "tea-target" }];
    rerender();
    expect(mocks.select).toHaveBeenCalledWith("tea-target");
    expect(opened).toHaveBeenCalledOnce();
  });
  it("opens an email-redeemed membership without replaying acceptance", async () => {
    mocks.preview.data.workspaceInvitePreview.alreadyMember = true;
    const { result } = renderHook(() => useInviteRedemption(TOKEN, vi.fn()));
    await act(() => result.current.accept());
    expect(mocks.mutate).not.toHaveBeenCalled();
    expect(mocks.select).toHaveBeenCalledWith("tea-target");
  });
  it("retries access without redeeming twice after a committed join", async () => {
    mocks.query.mockRejectedValueOnce(new Error("offline"));
    const opened = vi.fn();
    const { result } = renderHook(() => useInviteRedemption(TOKEN, opened));
    await act(() => result.current.accept());
    expect(result.current.errorKey).toBe("invites.accessPending");
    expect(mocks.select).not.toHaveBeenCalled();
    expect(opened).not.toHaveBeenCalled();
    await act(() => result.current.accept());
    expect(mocks.mutate).toHaveBeenCalledOnce();
    expect(opened).toHaveBeenCalledOnce();
  });
  it("keeps the current workspace when the joined membership is not visible yet", async () => {
    mocks.query.mockResolvedValueOnce({
      data: { workspaces: [{ id: "tea-personal" }] },
    });
    const { result } = renderHook(() => useInviteRedemption(TOKEN, vi.fn()));
    await act(() => result.current.accept());
    expect(result.current.errorKey).toBe("invites.accessPending");
    expect(mocks.select).not.toHaveBeenCalled();
  });
  it("opens a membership established after preview without an already-used error", async () => {
    mocks.mutate.mockRejectedValueOnce(
      new CombinedGraphQLErrors({
        errors: [
          { message: "used", extensions: { code: "INVITE_ALREADY_ACCEPTED" } },
        ],
      }),
    );
    mocks.previewRefetch.mockResolvedValueOnce({
      data: {
        workspaceInvitePreview: {
          workspaceId: "tea-target",
          alreadyMember: true,
        },
      },
    });
    const opened = vi.fn();
    const { result } = renderHook(() => useInviteRedemption(TOKEN, opened));
    await act(() => result.current.accept());
    expect(result.current.errorKey).toBeNull();
    expect(opened).toHaveBeenCalledOnce();
    expect(mocks.select).toHaveBeenCalledWith("tea-target");
  });
  it("handles an unavailable membership refresh after ambiguous acceptance", async () => {
    mocks.mutate.mockRejectedValueOnce(
      new CombinedGraphQLErrors({
        errors: [
          { message: "used", extensions: { code: "INVITE_ALREADY_ACCEPTED" } },
        ],
      }),
    );
    mocks.previewRefetch.mockRejectedValueOnce(new TypeError("offline"));
    const { result } = renderHook(() => useInviteRedemption(TOKEN, vi.fn()));
    await act(() => result.current.accept());
    expect(result.current.errorKey).toBe("invites.retryError");
    expect(mocks.select).not.toHaveBeenCalled();
  });
  it("waits for effective authorization after membership commits", async () => {
    mocks.query.mockResolvedValueOnce({
      data: { workspaces: [{ id: "tea-target" }] },
    });
    mocks.query.mockResolvedValueOnce({
      data: { viewerCapabilities: { canView: false } },
    });
    const opened = vi.fn();
    const { result } = renderHook(() => useInviteRedemption(TOKEN, opened));
    await act(() => result.current.accept());
    expect(result.current.errorKey).toBe("invites.accessPending");
    expect(opened).not.toHaveBeenCalled();
    await act(() => result.current.accept());
    expect(mocks.mutate).toHaveBeenCalledOnce();
    expect(opened).toHaveBeenCalledOnce();
  });
  it("stops offering acceptance when an invitation expires after preview", async () => {
    mocks.mutate.mockRejectedValueOnce(
      new CombinedGraphQLErrors({
        errors: [
          { message: "expired", extensions: { code: "INVITE_EXPIRED" } },
        ],
      }),
    );
    const { result } = renderHook(() => useInviteRedemption(TOKEN, vi.fn()));
    await act(() => result.current.accept());
    expect(result.current.details).toBeNull();
    expect(result.current.retryable).toBe(false);
  });
  it("prevents double clicks and keeps Joining visible during the request", async () => {
    let finish!: (value: unknown) => void;
    mocks.mutate.mockReturnValue(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    const { result } = renderHook(() => useInviteRedemption(TOKEN, vi.fn()));
    let pending!: Promise<void>;
    act(() => {
      pending = result.current.accept();
      void result.current.accept();
    });
    expect(result.current.busy).toBe(true);
    expect(mocks.mutate).toHaveBeenCalledOnce();
    await act(async () => {
      finish({
        data: { acceptWorkspaceInvite: { workspaceId: "tea-target" } },
      });
      await pending;
    });
    expect(result.current.busy).toBe(false);
  });
  it.each([
    [new TypeError("offline"), "invites.retryError"],
    [
      new CombinedGraphQLErrors({
        errors: [
          { message: "expired", extensions: { code: "INVITE_EXPIRED" } },
        ],
      }),
      "invites.expired",
    ],
    [
      new CombinedGraphQLErrors({
        errors: [
          { message: "seats", extensions: { code: "INVITE_PLAN_LIMIT" } },
        ],
      }),
      "invites.planLimit",
    ],
  ])(
    "shows inline errors and preserves the invitation for recovery",
    async (error, key) => {
      mocks.mutate.mockRejectedValueOnce(error);
      const { result } = renderHook(() => useInviteRedemption(TOKEN, vi.fn()));
      await act(() => result.current.accept());
      expect(result.current.errorKey).toBe(key);
      expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBe(
        TOKEN,
      );
      expect(mocks.select).not.toHaveBeenCalled();
    },
  );
});

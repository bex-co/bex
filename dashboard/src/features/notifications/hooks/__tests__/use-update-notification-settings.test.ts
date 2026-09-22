import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-canary" }),
}));

const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { error: (...a: unknown[]) => toastError(...a), success: vi.fn() },
}));

import { useUpdateNotificationSettings } from "@/features/notifications/hooks/use-update-notification-settings";
import { NotificationSettingsDocument } from "@/graphql/definitions";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastError.mockReset();
});

describe("useUpdateNotificationSettings", () => {
  it("writes the mutation response straight into the query's cache entry, not a refetch", () => {
    mockUseMutation.mockReturnValue([vi.fn()]);
    renderHook(() => useUpdateNotificationSettings());

    const [, options] = mockUseMutation.mock.calls[0] as [
      unknown,
      {
        update: (
          cache: unknown,
          result: unknown,
          context: { variables?: { ownerId?: string } },
        ) => void;
      },
    ];
    const writeQuery = vi.fn();
    const fakeCache = { writeQuery };
    options.update(
      fakeCache,
      {
        data: {
          updateNotificationSettings: {
            __typename: "NotificationSettings",
            deployStarted: true,
            deploySucceeded: false,
            deployFailed: true,
          },
        },
      },
      { variables: { ownerId: "tea-canary" } },
    );

    // The cache entry is keyed by workspace (w4/m128): writing it unkeyed
    // would file one workspace's answer where another's belongs.
    expect(writeQuery).toHaveBeenCalledWith({
      query: NotificationSettingsDocument,
      variables: { ownerId: "tea-canary" },
      data: {
        notificationSettings: {
          __typename: "NotificationSettings",
          deployStarted: true,
          deploySucceeded: false,
          deployFailed: true,
        },
      },
    });
  });

  it("the cache-write callback is a no-op when the mutation returned no data", () => {
    mockUseMutation.mockReturnValue([vi.fn()]);
    renderHook(() => useUpdateNotificationSettings());

    const [, options] = mockUseMutation.mock.calls[0] as [
      unknown,
      { update: (cache: unknown, result: unknown, context: object) => void },
    ];
    const writeQuery = vi.fn();
    options.update({ writeQuery }, { data: undefined }, {});

    expect(writeQuery).not.toHaveBeenCalled();
  });
  it("sends all three preference fields plus the current workspace, and resolves true", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdateNotificationSettings());
    let ok;
    await act(async () => {
      ok = await result.current.update({
        deployStarted: false,
        deploySucceeded: false,
        deployFailed: true,
      });
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: {
        deployStarted: false,
        deploySucceeded: false,
        deployFailed: true,
        ownerId: "tea-canary",
      },
    });
    expect(toastError).not.toHaveBeenCalled();
  });

  it("toasts and resolves false on a mutation failure", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("boom"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdateNotificationSettings());
    let ok;
    await act(async () => {
      ok = await result.current.update({
        deployStarted: true,
        deploySucceeded: true,
        deployFailed: true,
      });
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalled();
  });
});

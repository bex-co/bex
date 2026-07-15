import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
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
      { update: (cache: unknown, result: unknown) => void },
    ];
    const writeQuery = vi.fn();
    const fakeCache = { writeQuery };
    options.update(fakeCache, {
      data: {
        updateNotificationSettings: {
          __typename: "NotificationSettings",
          deployStarted: true,
          deploySucceeded: false,
          deployFailed: true,
        },
      },
    });

    expect(writeQuery).toHaveBeenCalledWith({
      query: NotificationSettingsDocument,
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
      { update: (cache: unknown, result: unknown) => void },
    ];
    const writeQuery = vi.fn();
    options.update({ writeQuery }, { data: undefined });

    expect(writeQuery).not.toHaveBeenCalled();
  });
  it("sends all three preference fields as mutation variables and resolves true", async () => {
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

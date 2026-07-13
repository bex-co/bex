import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

import { useNotificationSettings } from "@/features/notifications/hooks/use-notification-settings";

beforeEach(() => {
  mockUseQuery.mockReset();
});

describe("useNotificationSettings", () => {
  it("maps notificationSettings to the view", () => {
    mockUseQuery.mockReturnValue({
      data: {
        notificationSettings: {
          __typename: "NotificationSettings",
          deploySucceeded: false,
          deployFailed: true,
        },
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useNotificationSettings());
    expect(result.current.settings).toEqual({
      deploySucceeded: false,
      deployFailed: true,
    });
  });

  it("defaults to both true when no row exists yet (null response)", () => {
    mockUseQuery.mockReturnValue({
      data: { notificationSettings: null },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useNotificationSettings());
    expect(result.current.settings).toEqual({
      deploySucceeded: true,
      deployFailed: true,
    });
  });

  it("defaults to both true while loading with no data yet", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useNotificationSettings());
    expect(result.current.settings).toEqual({
      deploySucceeded: true,
      deployFailed: true,
    });
  });
});

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useServiceNotifications } from "../use-service-notifications";

const { mutate, success, error } = vi.hoisted(() => ({
  mutate: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
}));
vi.mock("@apollo/client/react", () => ({ useMutation: () => [mutate] }));
vi.mock("sonner", () => ({ toast: { success, error } }));
vi.mock("@/common/hooks/use-translations", () => ({
  useTranslations: () => ({ t: (key: string) => key }),
}));
vi.mock("@/graphql/definitions", () => ({
  SetNotificationsToSendDocument: {},
}));

describe("useServiceNotifications", () => {
  beforeEach(() => {
    mutate.mockReset();
    success.mockReset();
    error.mockReset();
  });
  it("writes the selected policy", async () => {
    mutate.mockResolvedValue({});
    const { result } = renderHook(() => useServiceNotifications());
    await act(async () => {
      expect(await result.current.setNotificationsToSend("srv-1", "none")).toBe(
        true,
      );
    });
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "srv-1", value: "none" },
    });
    expect(success).toHaveBeenCalled();
  });
  it("reports failures", async () => {
    mutate.mockRejectedValue(new Error("nope"));
    const { result } = renderHook(() => useServiceNotifications());
    await act(async () => {
      expect(await result.current.setNotificationsToSend("srv-1", "all")).toBe(
        false,
      );
    });
    expect(error).toHaveBeenCalled();
  });
});

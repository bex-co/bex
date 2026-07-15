import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

import { useMaintenanceMode } from "@/features/services/hooks/use-maintenance-mode";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useMaintenanceMode", () => {
  it("sends the complete two-key mutation and reports success", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useMaintenanceMode());

    await act(async () => {
      expect(
        await result.current.setMaintenanceMode(
          "srv-web",
          true,
          "https://status.example.com/maintenance",
        ),
      ).toBe(true);
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: {
        id: "srv-web",
        maintenanceMode: {
          enabled: true,
          uri: "https://status.example.com/maintenance",
        },
      },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Maintenance mode enabled.");
    expect(toastError).not.toHaveBeenCalled();
  });

  it("returns false and never shows false success when validation fails", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(new Error("maintenanceMode.uri is invalid")),
    ]);
    const { result } = renderHook(() => useMaintenanceMode());

    await act(async () => {
      expect(
        await result.current.setMaintenanceMode(
          "srv-web",
          true,
          "https://srv-web.onbex.co/self",
        ),
      ).toBe(false);
    });

    expect(toastError).toHaveBeenCalledWith(
      "Could not update maintenance mode. Try again.",
    );
    expect(toastSuccess).not.toHaveBeenCalled();
  });
});

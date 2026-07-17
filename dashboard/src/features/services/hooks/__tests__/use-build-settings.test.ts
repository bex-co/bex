import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";

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

import { useStartCommand } from "@/features/services/hooks/use-start-command";
import { useBuildCommand } from "@/features/services/hooks/use-build-command";
import { useDockerfilePath } from "@/features/services/hooks/use-dockerfile-path";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("Build & Deploy setting hooks", () => {
  it("persists a start command through setStartCommand", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useStartCommand());

    await act(async () => {
      expect(await result.current.setStartCommand("srv-1", "npm start")).toBe(
        true,
      );
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "srv-1", command: "npm start" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Start Command updated.");
  });

  it("reports a start-command mutation failure", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(new Error("forbidden")),
    ]);
    const { result } = renderHook(() => useStartCommand());

    await act(async () => {
      expect(await result.current.setStartCommand("srv-1", "npm start")).toBe(
        false,
      );
    });
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't update the Start Command. Please try again.",
    );
  });

  it("persists a Dockerfile path and allows clearing it", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useDockerfilePath());

    await act(async () => {
      expect(
        await result.current.setDockerfilePath(
          "srv-1",
          "docker/Dockerfile.prod",
        ),
      ).toBe(true);
      await result.current.setDockerfilePath("srv-1", "");
    });

    expect(mutate).toHaveBeenNthCalledWith(1, {
      variables: {
        id: "srv-1",
        dockerfilePath: "docker/Dockerfile.prod",
      },
    });
    expect(mutate).toHaveBeenNthCalledWith(2, {
      variables: { id: "srv-1", dockerfilePath: "" },
    });
  });

  it("reports a Dockerfile-path mutation failure", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(new Error("invalid path")),
    ]);
    const { result } = renderHook(() => useDockerfilePath());

    await act(async () => {
      expect(
        await result.current.setDockerfilePath("srv-1", "../Dockerfile"),
      ).toBe(false);
    });
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't update the Dockerfile Path. Please try again.",
    );
  });

  it("persists a build command through setBuildCommand (w7/m41)", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useBuildCommand());

    await act(async () => {
      expect(
        await result.current.setBuildCommand("site-1", "npm run build"),
      ).toBe(true);
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "site-1", command: "npm run build" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Build Command updated.");
  });

  it("reports a build-command mutation failure", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(new Error("not allowed")),
    ]);
    const { result } = renderHook(() => useBuildCommand());

    await act(async () => {
      expect(await result.current.setBuildCommand("site-1", "bad")).toBe(false);
    });
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't update the Build Command. Please try again.",
    );
  });
});

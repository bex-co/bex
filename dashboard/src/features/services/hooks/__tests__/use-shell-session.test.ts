import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

import {
  ShellUnavailableError,
  useShellSession,
} from "@/features/services/hooks/use-shell-session";

beforeEach(() => mockUseMutation.mockReset());

// An unconfigured browser shell is recognized by its code, SHELL_UNAVAILABLE,
// not by "not configured" in its wording (w5/m130).
describe("useShellSession", () => {
  it("reports an unconfigured shell as ShellUnavailableError", async () => {
    mockUseMutation.mockReturnValue([
      vi
        .fn()
        .mockRejectedValue(
          codedGraphQLError("SHELL_UNAVAILABLE", {}, "reworded by the server"),
        ),
    ]);
    const { result } = renderHook(() => useShellSession());
    await expect(
      result.current.createShellSession("web"),
    ).rejects.toBeInstanceOf(ShellUnavailableError);
  });

  it("passes the wording alone through untouched", async () => {
    const worded = uncodedGraphQLError("web shell transport not configured");
    mockUseMutation.mockReturnValue([vi.fn().mockRejectedValue(worded)]);
    const { result } = renderHook(() => useShellSession());
    await expect(result.current.createShellSession("web")).rejects.toBe(worded);
  });
});

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";

const mutate = vi.fn();
const mockUseMutation = vi.fn((..._args: unknown[]) => [mutate]);
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

import { useCreateEnvironment } from "@/features/environments/hooks/use-create-environment";

beforeEach(() => {
  mutate.mockReset();
  mockUseMutation.mockClear();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useCreateEnvironment", () => {
  it("returns the created environment's immutable id", async () => {
    mutate.mockResolvedValue({
      data: { createEnvironment: { id: "env-returned" } },
    });
    const { result } = renderHook(() => useCreateEnvironment("prj-owner"));

    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.create("staging");
    });

    expect(id).toBe("env-returned");
    expect(mutate).toHaveBeenCalledWith({
      variables: { name: "staging", projectId: "prj-owner" },
    });
    expect(toastSuccess).toHaveBeenCalled();
  });

  it("keeps a missing-id response in the failure path", async () => {
    mutate.mockResolvedValue({ data: { createEnvironment: { id: null } } });
    const { result } = renderHook(() => useCreateEnvironment("prj-owner"));

    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.create("staging");
    });

    expect(id).toBeNull();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalled();
  });

  // w6/m49: the original bare `catch {}` bound no error variable at all —
  // every failure got the identical generic toast. This pins the fix: a
  // duplicate-name conflict shows the backend's specific reason instead.
  it("shows the backend's specific reason on a name conflict", async () => {
    mutate.mockRejectedValue(
      new CombinedGraphQLErrors({
        data: null,
        errors: [
          {
            message: 'an environment named "staging" already exists in this project',
            extensions: { code: "CONFLICT" },
          },
        ],
      }),
    );
    const { result } = renderHook(() => useCreateEnvironment("prj-owner"));

    await act(async () => {
      await result.current.create("staging");
    });

    expect(toastError).toHaveBeenCalledWith(
      'An environment named "staging" already exists in this project',
    );
  });

  it("falls through to the generic toast for a non-conflict error", async () => {
    mutate.mockRejectedValue(new Error("network error"));
    const { result } = renderHook(() => useCreateEnvironment("prj-owner"));

    await act(async () => {
      await result.current.create("staging");
    });

    expect(toastError).toHaveBeenCalledWith(expect.stringContaining("staging"));
  });
});

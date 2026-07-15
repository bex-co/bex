import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const createMutation = vi.fn();
const deleteMutation = vi.fn();
const refetch = vi.fn();
const mockUseQuery = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: (document: {
    definitions?: Array<{ name?: { value?: string } }>;
  }) => {
    const operation = document.definitions?.[0]?.name?.value;
    return operation === "CreateSSHKey"
      ? [createMutation]
      : [deleteMutation];
  },
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

vi.mock("@/common/hooks/use-translations", () => ({
  useTranslations: () => ({
    t: (key: string, values?: Record<string, string>) =>
      values?.name ? `${key}:${values.name}` : key,
  }),
}));

import { useSSHKeys } from "@/features/ssh-keys/hooks/use-ssh-keys";

beforeEach(() => {
  createMutation.mockReset();
  deleteMutation.mockReset();
  refetch.mockReset().mockResolvedValue({});
  toastSuccess.mockReset();
  toastError.mockReset();
  mockUseQuery.mockReset().mockReturnValue({
    data: {
      sshKeys: [
        null,
        {
          id: "ssk-d5t5d4v8g3c73f5m9peg",
          name: "workstation",
          publicKey: "ssh-ed25519 AAAATEST",
          fingerprint: "SHA256:example",
          createdAt: "2026-07-14T12:00:00Z",
        },
      ],
    },
    loading: false,
    error: undefined,
    refetch,
  });
});

describe("useSSHKeys", () => {
  it("maps only complete query rows", () => {
    const { result } = renderHook(() => useSSHKeys());

    expect(result.current.keys).toEqual([
      {
        id: "ssk-d5t5d4v8g3c73f5m9peg",
        name: "workstation",
        publicKey: "ssh-ed25519 AAAATEST",
        fingerprint: "SHA256:example",
        createdAt: "2026-07-14T12:00:00Z",
      },
    ]);
  });

  it("creates, refetches, and reports success", async () => {
    createMutation.mockResolvedValue({ data: { createSSHKey: { id: "ssk-1" } } });
    const { result } = renderHook(() => useSSHKeys());

    let ok = false;
    await act(async () => {
      ok = await result.current.create(
        "laptop",
        "ssh-ed25519 AAAATEST",
      );
    });

    expect(ok).toBe(true);
    expect(createMutation).toHaveBeenCalledWith({
      variables: { name: "laptop", publicKey: "ssh-ed25519 AAAATEST" },
    });
    expect(refetch).toHaveBeenCalledOnce();
    expect(toastSuccess).toHaveBeenCalledWith(
      "sshKeys.createSuccess:laptop",
    );
    expect(result.current.busy).toBeNull();
  });

  it("keeps duplicate and generic create failures actionable", async () => {
    const { result } = renderHook(() => useSSHKeys());

    createMutation.mockRejectedValueOnce(
      new Error("SSH public key is already registered"),
    );
    await act(async () => {
      expect(
        await result.current.create("duplicate", "ssh-ed25519 AAAATEST"),
      ).toBe(false);
    });
    expect(toastError).toHaveBeenLastCalledWith("sshKeys.duplicateError");

    createMutation.mockRejectedValueOnce(new Error("forbidden"));
    await act(async () => {
      expect(
        await result.current.create("blocked", "ssh-ed25519 AAAATEST"),
      ).toBe(false);
    });
    expect(toastError).toHaveBeenLastCalledWith("sshKeys.createError");
    expect(refetch).not.toHaveBeenCalled();
  });

  it("deletes and refetches on success but preserves the row on failure", async () => {
    const { result } = renderHook(() => useSSHKeys());

    deleteMutation.mockResolvedValueOnce({ data: { deleteSSHKey: true } });
    await act(async () => {
      expect(await result.current.remove("ssk-1", "laptop")).toBe(true);
    });
    expect(deleteMutation).toHaveBeenCalledWith({ variables: { id: "ssk-1" } });
    expect(refetch).toHaveBeenCalledOnce();
    expect(toastSuccess).toHaveBeenLastCalledWith(
      "sshKeys.deleteSuccess:laptop",
    );

    refetch.mockClear();
    deleteMutation.mockRejectedValueOnce(new Error("forbidden"));
    await act(async () => {
      expect(await result.current.remove("ssk-2", "desktop")).toBe(false);
    });
    expect(refetch).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenLastCalledWith("sshKeys.deleteError");
    expect(result.current.keys).toHaveLength(1);
    expect(result.current.busy).toBeNull();
  });
});

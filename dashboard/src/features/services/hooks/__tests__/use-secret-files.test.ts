import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();
const mockClientQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
  useApolloClient: () => ({ query: mockClientQuery }),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...a: unknown[]) => toastSuccess(...a),
    error: (...a: unknown[]) => toastError(...a),
  },
}));

import {
  useSecretFileNames,
  useRevealSecretFile,
  useSecretFileMutations,
  classifySecretFileError,
} from "@/features/services/hooks/use-secret-files";

beforeEach(() => {
  mockUseQuery.mockReset();
  mockUseMutation.mockReset();
  mockClientQuery.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useSecretFileNames", () => {
  it("maps the nested secretFileNames to a names-only list, dropping nulls; id falls back to name", () => {
    mockUseQuery.mockReturnValue({
      data: {
        service: {
          __typename: "Service",
          id: "web",
          secretFileNames: [
            { __typename: "SecretFile", id: "cert.pem", name: "cert.pem" },
            { __typename: "SecretFile", id: null, name: "key.pem" },
            null,
            { __typename: "SecretFile", id: "x", name: null },
          ],
        },
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useSecretFileNames("web"));
    expect(result.current.names).toEqual([
      { id: "cert.pem", name: "cert.pem" },
      { id: "key.pem", name: "key.pem" }, // id defaulted to name
    ]);
  });

  it("returns an empty list (not a crash) when the service is null", () => {
    mockUseQuery.mockReturnValue({
      data: { service: null },
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useSecretFileNames("web"));
    expect(result.current.names).toEqual([]);
  });
});

describe("useRevealSecretFile", () => {
  it("fetches one file's content on demand with the right variables and returns it", async () => {
    mockClientQuery.mockResolvedValue({
      data: { service: { secretFile: { content: "body" } } },
    });
    const { result } = renderHook(() => useRevealSecretFile("web"));
    const value = await result.current("cert.pem");
    expect(value).toBe("body");
    expect(mockClientQuery).toHaveBeenCalledWith(
      expect.objectContaining({ variables: { id: "web", name: "cert.pem" } }),
    );
  });

  it("returns '' when the content is absent", async () => {
    mockClientQuery.mockResolvedValue({
      data: { service: { secretFile: null } },
    });
    const { result } = renderHook(() => useRevealSecretFile("web"));
    expect(await result.current("cert.pem")).toBe("");
  });
});

describe("useSecretFileMutations", () => {
  it("setFile writes, refetches, and toasts success (with the redeploy note)", async () => {
    const setSecretFile = vi.fn().mockResolvedValue({});
    mockUseMutation.mockImplementation(() => [setSecretFile]);
    const refetch = vi.fn().mockResolvedValue([]);

    const { result } = renderHook(() =>
      useSecretFileMutations("web", refetch),
    );
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.setFile("cert.pem", "body");
    });

    expect(ok).toBe(true);
    expect(setSecretFile).toHaveBeenCalledWith({
      variables: { serviceId: "web", name: "cert.pem", content: "body" },
    });
    expect(refetch).toHaveBeenCalled();
    expect(toastSuccess).toHaveBeenCalledWith(
      "Saved cert.pem",
      expect.objectContaining({
        description: "The service is redeploying to apply the change.",
      }),
    );
  });

  it("setFile surfaces a failure as an error toast and resolves false", async () => {
    const setSecretFile = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockImplementation(() => [setSecretFile]);
    const refetch = vi.fn().mockResolvedValue([]);

    const { result } = renderHook(() =>
      useSecretFileMutations("web", refetch),
    );
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.setFile("cert.pem", "body");
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith("Couldn't save cert.pem");
    expect(refetch).not.toHaveBeenCalled();
  });
});

describe("classifySecretFileError", () => {
  it("classifies the store-unavailable, forbidden, and generic errors, and null", () => {
    expect(classifySecretFileError(undefined)).toBeNull();
    expect(
      classifySecretFileError(new Error("secret store not configured")),
    ).toBe("unavailable");
    expect(classifySecretFileError(new Error("forbidden"))).toBe("forbidden");
    expect(classifySecretFileError(new Error("boom"))).toBe("generic");
  });
});

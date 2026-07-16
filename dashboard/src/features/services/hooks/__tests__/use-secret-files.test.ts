import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";

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
  it("maps the paged secret-file envelope to a names-only list, dropping nulls; id falls back to name", () => {
    mockUseQuery.mockReturnValue({
      data: {
        secretFiles: [
          {
            secretFile: {
              __typename: "SecretFileListValue",
              id: "cert.pem",
              name: "cert.pem",
            },
            cursor: "cert.pem",
          },
          {
            secretFile: {
              __typename: "SecretFileListValue",
              id: null,
              name: "key.pem",
            },
            cursor: "key.pem",
          },
          null,
          {
            secretFile: {
              __typename: "SecretFileListValue",
              id: "x",
              name: null,
            },
            cursor: "x",
          },
        ],
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

  it("returns an empty list (not a crash) when the list is null", () => {
    mockUseQuery.mockReturnValue({
      data: { secretFiles: null },
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useSecretFileNames("web"));
    expect(result.current.names).toEqual([]);
  });

  it("walks every cursor page and appends each name once", async () => {
    const first = Array.from({ length: 100 }, (_, i) => {
      const name = `file_${String(i).padStart(3, "0")}`;
      return { secretFile: { id: name, name }, cursor: name };
    });
    mockUseQuery.mockReturnValue({
      data: { secretFiles: first },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    mockClientQuery.mockResolvedValue({
      data: {
        secretFiles: [
          { secretFile: { id: "file_100", name: "file_100" }, cursor: "file_100" },
          { secretFile: { id: "file_101", name: "file_101" }, cursor: "file_101" },
        ],
      },
    });

    const { result } = renderHook(() => useSecretFileNames("web"));
    await waitFor(() => expect(result.current.names).toHaveLength(102));
    expect(new Set(result.current.names.map((n) => n.id)).size).toBe(102);
    expect(mockClientQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: { serviceId: "web", cursor: "file_099", limit: 100 },
      }),
    );
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

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";

const useQueryMock = vi.fn();
const mutateMock = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => useQueryMock(...args),
  useMutation: () => [mutateMock],
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-owner" }),
}));

import {
  useSandboxGroupCredentialMutations,
  useSandboxGroupCredentials,
} from "@/features/sandbox-groups/hooks/use-sandbox-group-credentials";

const CRED = {
  __typename: "SandboxGroupCredential" as const,
  id: "openai-key",
  domain: "api.openai.com",
  enabled: true,
  methods: ["POST"],
  paths: ["/v1/**"],
  injection: {
    __typename: "SandboxGroupCredentialInjection" as const,
    type: "header",
    name: "Authorization",
    template: "Bearer ${SECRET}",
  },
  version: {
    __typename: "SandboxGroupCredentialVersion" as const,
    id: 1,
    updatedAt: "2026-11-20T00:00:00Z",
    updatedBy: "usr-admin",
    fingerprint: "a".repeat(64),
    restoredFromVersionId: null,
  },
};

beforeEach(() => {
  useQueryMock.mockReset();
  mutateMock.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useSandboxGroupCredentials", () => {
  it("keeps loading false when cached data is already present during background refresh", () => {
    useQueryMock.mockReturnValue({
      data: { sandboxGroupCredentials: [CRED] },
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() =>
      useSandboxGroupCredentials("sbg-1", "tea-owner"),
    );
    expect(result.current.loading).toBe(false);
    expect(result.current.credentials).toEqual([CRED]);
  });

  it("flags unavailable when OpenBao credential storage returns SANDBOX_CREDENTIALS_UNAVAILABLE", () => {
    useQueryMock.mockReturnValue({
      data: undefined,
      loading: false,
      error: codedGraphQLError("SANDBOX_CREDENTIALS_UNAVAILABLE"),
      refetch: vi.fn(),
    });
    const { result } = renderHook(() =>
      useSandboxGroupCredentials("sbg-1", "tea-owner"),
    );
    expect(result.current.unavailable).toBe(true);
  });
});

describe("useSandboxGroupCredentialMutations", () => {
  it("maps duplicate ID and busy errors to localized messages", async () => {
    mutateMock.mockRejectedValueOnce(
      codedGraphQLError("SANDBOX_CREDENTIAL_ALREADY_EXISTS"),
    );
    const onChanged = vi.fn();
    const { result } = renderHook(() =>
      useSandboxGroupCredentialMutations("sbg-1", "tea-owner", onChanged),
    );

    let res: Awaited<ReturnType<typeof result.current.createCredential>> = {
      ok: true,
      data: null,
      errorMessage: null,
    };
    await act(async () => {
      res = await result.current.createCredential({
        id: "openai-key",
        domain: "api.openai.com",
        value: "sk-secret",
        injection: { type: "header", name: "Authorization" },
      });
    });
    expect(res.ok).toBe(false);
    expect(res.errorMessage).toContain("openai-key");
    expect(onChanged).not.toHaveBeenCalled();

    mutateMock.mockRejectedValueOnce(
      codedGraphQLError("SANDBOX_CREDENTIAL_CONFLICT"),
    );
    await act(async () => {
      res = await result.current.updateCredential("openai-key", {
        enabled: false,
      });
    });
    expect(res.ok).toBe(false);
    expect(res.errorMessage).toContain("progress");
  });

  it("restores and deletes credentials and invokes onChanged", async () => {
    const restoredCred = {
      ...CRED,
      version: { ...CRED.version, id: 2, restoredFromVersionId: 1 },
    };
    mutateMock.mockResolvedValueOnce({
      data: { restoreSandboxGroupCredential: restoredCred },
    });
    mutateMock.mockResolvedValueOnce({
      data: { deleteSandboxGroupCredential: true },
    });
    const onChanged = vi.fn();
    const { result } = renderHook(() =>
      useSandboxGroupCredentialMutations("sbg-1", "tea-owner", onChanged),
    );

    await act(async () => {
      const restored = await result.current.restoreCredential("openai-key", 1);
      expect(restored).toEqual(restoredCred);
    });
    expect(toastSuccess).toHaveBeenCalledWith(
      "Restored openai-key from version v1.",
    );

    await act(async () => {
      const ok = await result.current.deleteCredential("openai-key");
      expect(ok).toBe(true);
    });
    expect(onChanged).toHaveBeenCalledTimes(2);
  });
});

import { useLayoutEffect } from "react";
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  CancelWorkspaceCreationDocument,
  FinalizeWorkspaceCreationDocument,
  PrepareWorkspaceCreationDocument,
  RenameWorkspaceDocument,
  WorkspaceCreationAttemptDocument,
} from "@/graphql/definitions";
import type { WorkspacePlanId } from "@/features/workspaces/types";

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import i18n from "@/i18n/init";
import { uncodedGraphQLError } from "@/test/mocks/apollo";
import { useWorkspaceCreationBilling } from "@/features/workspaces/hooks/use-workspace-creation-billing";

function attemptRow(overrides: Record<string, unknown> = {}) {
  return {
    __typename: "WorkspaceCreationAttempt" as const,
    id: "wca-1",
    name: "acme",
    plan: "pro",
    billingEmail: "billing@acme.test",
    paymentRequired: true,
    state: "prepared",
    clientSecret: "seti_secret",
    publishableKey: "pk_test",
    ...overrides,
  };
}

const resolvedPolicy = {
  data: {
    workspaceCreationPolicy: {
      mode: "on",
      paymentRequired: true,
      providerAvailable: true,
    },
  },
  loading: false,
  error: undefined,
};
let policyRead: { data?: unknown; loading: boolean; error?: Error };
let resumeRead: { data?: unknown; loading: boolean };
const prepareMutation = vi.fn();
const cancelMutation = vi.fn();
const finalizeMutation = vi.fn();
const renameMutation = vi.fn();

beforeEach(() => {
  policyRead = resolvedPolicy;
  resumeRead = { data: undefined, loading: true };
  prepareMutation.mockReset();
  cancelMutation.mockReset();
  finalizeMutation.mockReset();
  renameMutation.mockReset();
  mockUseQuery.mockReset();
  mockUseQuery.mockImplementation((document: unknown) =>
    document === WorkspaceCreationAttemptDocument ? resumeRead : policyRead,
  );
  mockUseMutation.mockReset();
  const mutations = new Map<unknown, unknown>([
    [PrepareWorkspaceCreationDocument, prepareMutation],
    [CancelWorkspaceCreationDocument, cancelMutation],
    [FinalizeWorkspaceCreationDocument, finalizeMutation],
    [RenameWorkspaceDocument, renameMutation],
  ]);
  mockUseMutation.mockImplementation((document: unknown) => [
    mutations.get(document) ?? vi.fn(),
  ]);
});

describe("useWorkspaceCreationBilling", () => {
  it("resumes the attempt its read returns and prepares against it", async () => {
    const committed: (string | undefined)[] = [];
    const { result, rerender } = renderHook(() => {
      const value = useWorkspaceCreationBilling("hobby", "wca-1");
      useLayoutEffect(() => {
        committed.push(value.attempt?.id);
      });
      return value;
    });
    expect(result.current.attempt).toBeNull();

    resumeRead = {
      data: { workspaceCreationAttempt: attemptRow() },
      loading: false,
    };
    committed.length = 0;
    rerender();

    expect(result.current.attempt).toMatchObject({ id: "wca-1", plan: "pro" });
    // The read's attempt is on screen from the first commit that has it.
    expect(committed).toEqual(["wca-1"]);

    prepareMutation.mockResolvedValue({
      data: {
        prepareWorkspaceCreation: attemptRow({ state: "setup_pending" }),
      },
    });
    await act(async () => {
      await result.current.prepare("acme", "billing@acme.test", true);
    });
    expect(prepareMutation).toHaveBeenCalledWith({
      variables: {
        name: "acme",
        plan: "pro",
        billingEmail: "billing@acme.test",
        attemptId: "wca-1",
        collectPaymentMethod: true,
      },
    });
    expect(result.current.attempt?.state).toBe("setup_pending");
  });

  it("keeps the attempt and rejects when cancelling it fails, clears it once cancelled", async () => {
    resumeRead = {
      data: { workspaceCreationAttempt: attemptRow() },
      loading: false,
    };
    const { result } = renderHook(() =>
      useWorkspaceCreationBilling("hobby", "wca-1"),
    );
    expect(result.current.attempt?.id).toBe("wca-1");

    cancelMutation.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    await act(async () => {
      await expect(result.current.cancel()).rejects.toThrow("Failed to fetch");
    });
    expect(result.current.attempt?.id).toBe("wca-1");

    cancelMutation.mockResolvedValueOnce({});
    await act(async () => {
      await result.current.cancel();
    });
    expect(cancelMutation).toHaveBeenLastCalledWith({
      variables: { attemptId: "wca-1" },
    });
    expect(result.current.attempt).toBeNull();
  });

  // w3/m246: the freeform name the user chose is applied once the workspace
  // exists; a failed rename keeps the created workspace under its slug.
  it("names the finalized workspace, and keeps the slug if naming fails", async () => {
    resumeRead = {
      data: {
        workspaceCreationAttempt: attemptRow({ state: "setup_succeeded" }),
      },
      loading: false,
    };
    const created = {
      __typename: "Workspace",
      id: "tea-new",
      name: "acme",
      slug: "acme",
      plan: "pro",
      role: "admin",
      createdAt: null,
    };
    finalizeMutation.mockResolvedValue({
      data: { finalizeWorkspaceCreation: created },
    });
    renameMutation.mockResolvedValueOnce({
      data: { renameWorkspace: { ...created, name: "Acme Corp" } },
    });
    const { result } = renderHook(() =>
      useWorkspaceCreationBilling("pro", "wca-1"),
    );
    let ws: Awaited<ReturnType<typeof result.current.finalize>> = null;
    await act(async () => {
      ws = await result.current.finalize(result.current.attempt!, "Acme Corp");
    });
    expect(renameMutation).toHaveBeenCalledWith({
      variables: { id: "tea-new", name: "Acme Corp" },
    });
    expect(ws).toMatchObject({
      id: "tea-new",
      name: "Acme Corp",
      slug: "acme",
    });

    renameMutation.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    await act(async () => {
      ws = await result.current.finalize(result.current.attempt!, "Acme Corp");
    });
    expect(ws).toMatchObject({ id: "tea-new", name: "acme" });
  });

  it("drops the previous plan's error as soon as the plan changes", async () => {
    prepareMutation.mockRejectedValue(uncodedGraphQLError("card declined"));
    let plan: WorkspacePlanId = "pro";
    const committed: (string | null)[] = [];
    const { result, rerender } = renderHook(() => {
      const value = useWorkspaceCreationBilling(plan);
      useLayoutEffect(() => {
        committed.push(value.error);
      });
      return value;
    });
    await act(async () => {
      await result.current.prepare("acme", "billing@acme.test", true);
    });
    expect(result.current.error).toBe("Card declined");

    plan = "scale";
    committed.length = 0;
    rerender();

    expect(result.current.error).toBeNull();
    expect(committed).toEqual([null]);
  });

  // A plan-policy read that never reached bex-api has no reason to relay: the
  // banner reads the page's own copy in the user's language, while a refused
  // read still shows the server's sentence (w5/164).
  it("explains a failed policy read in the user's language", async () => {
    policyRead = {
      data: undefined,
      loading: false,
      error: new TypeError("Failed to fetch"),
    };
    const { result } = renderHook(() => useWorkspaceCreationBilling("pro"));
    expect(result.current.error).toBe(
      "Couldn't load workspace setup. Refresh the page to try again.",
    );

    await i18n.changeLanguage("zh");
    try {
      const zh = renderHook(() => useWorkspaceCreationBilling("pro"));
      expect(zh.result.current.error).toBe(
        "无法加载工作区创建信息。请刷新页面重试。",
      );
    } finally {
      await i18n.changeLanguage("en");
    }

    policyRead = {
      data: undefined,
      loading: false,
      error: uncodedGraphQLError("workspace creation is paused"),
    };
    const refused = renderHook(() => useWorkspaceCreationBilling("pro"));
    expect(refused.result.current.error).toBe("Workspace creation is paused");
  });
});

import { useLayoutEffect } from "react";
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  PrepareWorkspaceCreationDocument,
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

const policyRead = {
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
let resumeRead: { data?: unknown; loading: boolean };
const prepareMutation = vi.fn();

beforeEach(() => {
  resumeRead = { data: undefined, loading: true };
  prepareMutation.mockReset();
  mockUseQuery.mockReset();
  mockUseQuery.mockImplementation((document: unknown) =>
    document === WorkspaceCreationAttemptDocument ? resumeRead : policyRead,
  );
  mockUseMutation.mockReset();
  mockUseMutation.mockImplementation((document: unknown) => [
    document === PrepareWorkspaceCreationDocument ? prepareMutation : vi.fn(),
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

  it("drops the previous plan's error as soon as the plan changes", async () => {
    prepareMutation.mockRejectedValue(new Error("card declined"));
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
    expect(result.current.error).toBe("card declined");

    plan = "scale";
    committed.length = 0;
    rerender();

    expect(result.current.error).toBeNull();
    expect(committed).toEqual([null]);
  });
});

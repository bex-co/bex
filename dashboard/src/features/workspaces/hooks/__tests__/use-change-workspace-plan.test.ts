import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...a: unknown[]) => toastSuccess(...a),
    error: (...a: unknown[]) => toastError(...a),
  },
}));

import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";
import { useChangeWorkspacePlan } from "@/features/workspaces/hooks/use-change-workspace-plan";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

/** The hook as a plan page holds it, with that page's fallback. */
function hook(fallback = "Error updating plan.") {
  return renderHook(() => useChangeWorkspacePlan(fallback));
}

async function changeTo(
  result: ReturnType<typeof hook>["result"],
  plan: "hobby" | "pro" | "scale",
) {
  let ok: boolean | undefined;
  await act(async () => {
    ok = await result.current.changePlan("tea-1", plan);
  });
  return ok;
}

describe("useChangeWorkspacePlan (w3/m708)", () => {
  it("sends the plan and shows Render's plan-updated toast", async () => {
    const mutate = vi.fn().mockResolvedValue({ data: {} });
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = hook();

    expect(await changeTo(result, "pro")).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "tea-1", plan: "pro" },
    });
    expect(toastSuccess).toHaveBeenCalledWith(
      "Your workspace is now on the Pro plan.",
      { description: expect.any(String) },
    );
    expect(result.current.error).toBeNull();
  });

  it("welcomes a workspace to Scale, as Render's toast does", async () => {
    mockUseMutation.mockReturnValue([vi.fn().mockResolvedValue({ data: {} })]);
    const { result } = hook();

    await changeTo(result, "scale");
    expect(toastSuccess).toHaveBeenCalledWith("Welcome to the Scale plan.", {
      description: expect.any(String),
    });
  });

  it("says Render's card message when bex-api wants a payment method", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(codedGraphQLError("PAYMENT_REQUIRED")),
    ]);
    const { result } = hook();

    expect(await changeTo(result, "pro")).toBe(false);
    expect(result.current.error).toBe(
      "Add a valid card to upgrade to a paid plan.",
    );
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("relays the server's reason, else the page's own fallback", async () => {
    mockUseMutation.mockReturnValue([
      vi
        .fn()
        .mockRejectedValue(
          uncodedGraphQLError(
            "workspace has 2 members, exceeds hobby plan's limit of 1",
          ),
        ),
    ]);
    const { result } = hook();
    await changeTo(result, "hobby");
    expect(result.current.error).toBe(
      "Workspace has 2 members, exceeds hobby plan's limit of 1",
    );

    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(new Error("Failed to fetch")),
    ]);
    const { result: fallback } = hook(
      "Server error during submission. Please try again",
    );
    await changeTo(fallback, "pro");
    expect(fallback.current.error).toBe(
      "Server error during submission. Please try again",
    );
  });

  it("returns an org primary's unmet preconditions as blockers", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(
        codedGraphQLError("ORGANIZATION_DOWNGRADE_BLOCKED", {
          preconditions: [
            { kind: "secondary_workspaces", items: ["beta"] },
            { kind: "member_limit", items: ["dev@example.com:developer"], limit: 1 },
            { kind: "member_roles", items: ["vic@example.com:viewer", 7] },
            { kind: "inbound_ip_rules" },
            "not a precondition",
          ],
        }),
      ),
    ]);
    const { result } = hook();

    expect(await changeTo(result, "pro")).toBe(false);
    expect(result.current.error).toBe(
      "There are some errors in your selection. Please review and fix them to continue.",
    );
    expect(result.current.preconditions).toEqual([
      { kind: "secondary_workspaces", items: ["beta"], limit: null },
      { kind: "member_limit", items: ["dev@example.com:developer"], limit: 1 },
      { kind: "member_roles", items: ["vic@example.com:viewer"], limit: null },
      { kind: "inbound_ip_rules", items: [], limit: null },
    ]);
  });

  it("says a secondary's plan is changed on its primary", async () => {
    mockUseMutation.mockReturnValue([
      vi
        .fn()
        .mockRejectedValue(codedGraphQLError("ORGANIZATION_PLAN_ON_PRIMARY")),
    ]);
    const { result } = hook();

    await changeTo(result, "pro");
    expect(result.current.error).toBe(
      "This workspace's plan is its organization's plan. Change it on the organization's primary workspace.",
    );
  });
});

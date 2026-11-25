// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { toast } from "sonner";
import { useBillingEmail } from "@/features/usage/hooks/use-billing-email";
import {
  codedGraphQLError,
  createSuccessQueryResult,
} from "@/test/mocks/apollo";

const mockUseQuery = vi.fn();
const mockMutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: () => [mockMutate],
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const pro = {
  __typename: "WorkspaceBillingEmail" as const,
  workspaceId: "tea-a",
  email: "owner@example.com",
  configurable: true,
};

// w6/m627: the Billing page's billing email read and write.
describe("useBillingEmail", () => {
  beforeEach(() => {
    mockUseQuery.mockReset();
    mockMutate.mockReset();
    vi.mocked(toast.success).mockReset();
    vi.mocked(toast.error).mockReset();
    mockUseQuery.mockReturnValue(
      createSuccessQueryResult({ workspaceBillingEmail: pro }),
    );
  });

  it("reads the workspace's billing email", () => {
    const { result } = renderHook(() => useBillingEmail("tea-a"));
    expect(result.current.billingEmail).toEqual(pro);
    expect(mockUseQuery.mock.calls[0][1]).toMatchObject({
      variables: { workspaceId: "tea-a" },
      skip: false,
    });
  });

  it("saves the address and says where billing now goes", async () => {
    mockMutate.mockResolvedValue({
      data: {
        setWorkspaceBillingEmail: { ...pro, email: "finance@example.com" },
      },
    });
    const { result } = renderHook(() => useBillingEmail("tea-a"));
    let ok = false;
    await act(async () => {
      ok = await result.current.save("finance@example.com");
    });
    expect(ok).toBe(true);
    expect(mockMutate).toHaveBeenCalledWith({
      variables: { workspaceId: "tea-a", email: "finance@example.com" },
    });
    expect(toast.success).toHaveBeenCalledWith(
      "Billing email set to finance@example.com",
    );
  });

  it("reads a plan refusal by its code, in the member's language", async () => {
    mockMutate.mockRejectedValue(
      codedGraphQLError("PLAN_FEATURE_REQUIRED", {
        plan: "hobby",
        feature: "billingEmail",
        requiredPlan: "pro",
      }),
    );
    const { result } = renderHook(() => useBillingEmail("tea-a"));
    let ok = true;
    await act(async () => {
      ok = await result.current.save("finance@example.com");
    });
    expect(ok).toBe(false);
    expect(toast.error).toHaveBeenCalledWith(
      "Hobby workspaces use the owner's account email. A separate billing email is available on Pro and above.",
    );
  });

  it("relays any other refusal the server gave", async () => {
    mockMutate.mockRejectedValue(
      codedGraphQLError("BAD_REQUEST", {}, "billing email is invalid"),
    );
    const { result } = renderHook(() => useBillingEmail("tea-a"));
    await act(async () => {
      await result.current.save("not-an-address");
    });
    expect(toast.error).toHaveBeenCalledWith("Billing email is invalid");
  });
});

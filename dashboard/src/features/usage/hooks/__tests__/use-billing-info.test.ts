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
import {
  useBillingInfo,
  type BillingInfoInput,
  type SaveBillingInfoResult,
} from "@/features/usage/hooks/use-billing-info";
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

const unset = {
  __typename: "WorkspaceBillingInfo" as const,
  workspaceId: "tea-a",
  customerReady: true,
  billTo: "acme",
  companyName: "",
  address: null,
  taxId: null,
};

const input: BillingInfoInput = {
  companyName: "Acme GmbH",
  address: {
    line1: "Hauptstr. 1",
    line2: "",
    city: "Berlin",
    state: "",
    postalCode: "10115",
    country: "DE",
  },
  taxId: { type: "eu_vat", value: "DE123456789" },
};

/** Renders the hook and saves `input` through it. */
async function saveInput(): Promise<SaveBillingInfoResult> {
  const { result } = renderHook(() => useBillingInfo("tea-a"));
  let outcome: SaveBillingInfoResult = { ok: false, fieldError: null };
  await act(async () => {
    outcome = await result.current.save(input);
  });
  return outcome;
}

// w6/m628: the Billing page's billing info read and write.
describe("useBillingInfo", () => {
  beforeEach(() => {
    mockUseQuery.mockReset();
    mockMutate.mockReset();
    vi.mocked(toast.success).mockReset();
    vi.mocked(toast.error).mockReset();
    mockUseQuery.mockReturnValue(
      createSuccessQueryResult({ workspaceBillingInfo: unset }),
    );
  });

  it("reads the workspace's billing info", () => {
    const { result } = renderHook(() => useBillingInfo("tea-a"));
    expect(result.current.billingInfo).toEqual(unset);
    expect(mockUseQuery.mock.calls[0][1]).toMatchObject({
      variables: { workspaceId: "tea-a" },
      skip: false,
    });
  });

  it("sends the whole replacement and says whom invoices now bill", async () => {
    mockMutate.mockResolvedValue({
      data: {
        setWorkspaceBillingInfo: {
          ...unset,
          billTo: "Acme GmbH",
          companyName: "Acme GmbH",
        },
      },
    });
    expect(await saveInput()).toEqual({ ok: true });
    expect(mockMutate).toHaveBeenCalledWith({
      variables: { workspaceId: "tea-a", ...input },
    });
    expect(toast.success).toHaveBeenCalledWith(
      "Billing info saved. New invoices bill Acme GmbH.",
    );
  });

  it("returns a refused field with its reason for the form, without a toast", async () => {
    mockMutate.mockRejectedValue(
      codedGraphQLError(
        "BILLING_INFO_INVALID",
        { field: "taxId.value", reason: "Invalid value for eu_vat." },
        "taxId.value: Invalid value for eu_vat.",
      ),
    );
    expect(await saveInput()).toEqual({
      ok: false,
      fieldError: {
        field: "taxId.value",
        message: "Invalid value for eu_vat.",
      },
    });
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("toasts a refused field it cannot place", async () => {
    mockMutate.mockRejectedValue(
      codedGraphQLError(
        "BILLING_INFO_INVALID",
        { field: "phone" },
        "phone: is not accepted",
      ),
    );
    expect(await saveInput()).toEqual({ ok: false, fieldError: null });
    expect(toast.error).toHaveBeenCalledWith("Phone: is not accepted");
  });

  it("says a payment method comes first, in the member's language", async () => {
    mockMutate.mockRejectedValue(
      codedGraphQLError("BILLING_CUSTOMER_NOT_READY"),
    );
    expect(await saveInput()).toEqual({ ok: false, fieldError: null });
    expect(toast.error).toHaveBeenCalledWith(
      "Add a payment method first; billing info is kept with it.",
    );
  });

  it("falls back to its own words for a failure with no reason to relay", async () => {
    mockMutate.mockRejectedValue(new Error("Failed to fetch"));
    await saveInput();
    expect(toast.error).toHaveBeenCalledWith("Could not save the billing info");
  });
});

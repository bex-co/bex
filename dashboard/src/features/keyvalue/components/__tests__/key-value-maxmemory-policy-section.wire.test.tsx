import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { KeyValueMaxmemoryPolicySection } from "@/features/keyvalue/components/key-value-maxmemory-policy-section";

// Unlike the sibling section test (which mocks the hook and so bypasses the wire
// shape), this exercises the REAL hook against the underscored value bex-api
// actually returns — the w4/046 defect: the selector matched no hyphen-spelled
// option and rendered blank. Mock only the Apollo layer.
const refetch = vi.fn();
let policyData: { keyValue: { maxmemoryPolicy: string } | null } | undefined;

vi.mock("@apollo/client/react", () => ({
  useQuery: () => ({
    data: policyData,
    loading: false,
    error: undefined,
    refetch,
  }),
  useMutation: () => [vi.fn().mockResolvedValue({}), { loading: false }],
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

beforeEach(() => {
  refetch.mockReset();
  policyData = { keyValue: { maxmemoryPolicy: "allkeys_lfu" } };
});

describe("KeyValueMaxmemoryPolicySection (real hook, wire shape)", () => {
  it("shows the saved policy from the underscored API value, not a blank", () => {
    render(<KeyValueMaxmemoryPolicySection id="red-x" />);
    // Before the fix the disabled trigger was empty; now it renders the mapped
    // hyphen option so the user can read the saved policy.
    expect(
      screen.getByRole("combobox", { name: "Maxmemory policy" }),
    ).toHaveTextContent("allkeys-lfu");
  });

  it("treats the mapped policy as the baseline: Save is disabled until it changes", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<KeyValueMaxmemoryPolicySection id="red-x" />);

    await user.click(
      screen.getByRole("button", { name: "Edit maxmemory policy" }),
    );
    // The draft equals the normalized baseline, so no spurious dirty state.
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
  });

  it("keeps the selector blank for an absent read rather than guessing", () => {
    policyData = { keyValue: null };
    render(<KeyValueMaxmemoryPolicySection id="red-x" />);
    expect(
      screen.getByRole("combobox", { name: "Maxmemory policy" }),
    ).toHaveTextContent("");
  });
});

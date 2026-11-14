import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { OverlappingDeployPolicyCard } from "@/features/workspaces/components/overlapping-deploy-policy-card";

// The real hook runs; Apollo is faked, so the read and the mutation's
// variables are asserted at the GraphQL boundary.
const apollo = vi.hoisted(() => ({
  policy: "wait" as string | null,
  loading: false,
  mutate: vi.fn(),
}));
vi.mock("@apollo/client/react", () => ({
  useQuery: () => ({
    data: apollo.loading
      ? undefined
      : {
          workspace: { id: "tea-a", overlappingDeployPolicy: apollo.policy },
        },
    loading: apollo.loading,
  }),
  useMutation: () => [apollo.mutate],
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

// w4/653: Render's Workspace Settings → Overlapping Deploy Policy.
describe("OverlappingDeployPolicyCard", () => {
  beforeEach(() => {
    apollo.policy = "wait";
    apollo.loading = false;
    apollo.mutate.mockReset().mockResolvedValue({});
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canManage: true }),
    );
  });

  it("shows Wait selected and lets an Admin switch to Override", async () => {
    render(<OverlappingDeployPolicyCard workspaceId="tea-a" />);
    const wait = screen.getByRole("radio", { name: /^Wait/ });
    const override = screen.getByRole("radio", { name: /^Override/ });
    expect(wait).toHaveAttribute("aria-checked", "true");
    expect(
      screen.getByText(
        "Immediately cancel the in-progress deploy and start the new one.",
      ),
    ).toBeInTheDocument();
    const save = screen.getByRole("button", { name: "Save" });
    expect(save).toBeDisabled();
    await userEvent.click(override);
    expect(override).toHaveAttribute("aria-checked", "true");
    await userEvent.click(save);
    expect(apollo.mutate).toHaveBeenCalledWith({
      variables: { id: "tea-a", policy: "override" },
    });
  });

  it("is read-only for a member who cannot manage the workspace", () => {
    apollo.policy = "override";
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canManage: false }),
    );
    render(<OverlappingDeployPolicyCard workspaceId="tea-a" />);
    expect(screen.getByRole("radio", { name: /^Override/ })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(screen.getByRole("radio", { name: /^Wait/ })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
    expect(
      screen.getByText("Only workspace Admins can change this policy."),
    ).toBeInTheDocument();
  });

  it("previews its own geometry while the policy loads", () => {
    apollo.loading = true;
    render(<OverlappingDeployPolicyCard workspaceId="tea-a" />);
    expect(
      screen.getByTestId("overlapping-deploy-policy-skeleton"),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(2);
  });
});

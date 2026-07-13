import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChangePlanDialog } from "@/features/workspaces/components/change-plan-dialog";
import type { WorkspaceView } from "@/features/workspaces/types";

const changePlan = vi.fn();
vi.mock("@/features/workspaces/hooks/use-change-workspace-plan", () => ({
  useChangeWorkspacePlan: () => ({ changePlan, busy: false, error: mockError }),
}));

let mockError: string | null = null;

const WORKSPACE: WorkspaceView = {
  id: "tea-1",
  name: "acme-hq",
  plan: "pro",
  role: "admin",
  createdAt: null,
};

beforeEach(() => {
  changePlan.mockReset();
  changePlan.mockResolvedValue(true);
  mockError = null;
});

describe("ChangePlanDialog (w6/m12/t005)", () => {
  it("disables submit until a different plan is picked", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    render(
      <ChangePlanDialog
        workspace={WORKSPACE}
        open
        onOpenChange={vi.fn()}
        onChanged={onChanged}
      />,
    );

    const submit = screen.getByRole("button", { name: "Change Plan" });
    expect(submit).toBeDisabled();

    await user.click(screen.getByRole("radio", { name: /Scale/ }));
    expect(submit).toBeEnabled();

    await user.click(submit);
    expect(changePlan).toHaveBeenCalledWith("tea-1", "scale");
    expect(onChanged).toHaveBeenCalled();
  });

  it("surfaces a blocked downgrade's backend error inline, not just a toast", () => {
    mockError = "workspace has 2 members, exceeds hobby plan's limit of 1";
    render(
      <ChangePlanDialog
        workspace={WORKSPACE}
        open
        onOpenChange={vi.fn()}
        onChanged={vi.fn()}
      />,
    );

    expect(
      screen.getByText("workspace has 2 members, exceeds hobby plan's limit of 1"),
    ).toBeInTheDocument();
  });
});

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WorkspaceDetailsCard } from "@/features/workspaces/components/workspace-details-card";
import type { WorkspaceView } from "@/features/workspaces/types";

vi.mock("@/features/workspaces/hooks/use-rename-workspace", () => ({
  useRenameWorkspace: () => ({ rename: vi.fn(), busy: false, error: null }),
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ refetch: vi.fn() }),
}));

// Stand in for the real dialog: what matters here is whether the card opens it.
vi.mock("@/features/workspaces/components/change-plan-dialog", () => ({
  ChangePlanDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="change-plan-dialog" /> : null,
}));

const workspace: WorkspaceView = {
  id: "tea-1",
  name: "acme",
  plan: "hobby",
  role: "admin",
  createdAt: null,
};

describe("WorkspaceDetailsCard", () => {
  it("keeps the change-plan dialog closed when the route says it isn't open", () => {
    render(
      <WorkspaceDetailsCard
        workspace={workspace}
        changePlanOpen={false}
        onChangePlanOpenChange={vi.fn()}
      />,
    );
    expect(screen.queryByTestId("change-plan-dialog")).not.toBeInTheDocument();
  });

  // The far end of w6/m15/t001: the blocked-invite CTA navigates to
  // `/workspace/settings?plan=change`, and the route turns that into this prop.
  // Without it the CTA would land on the settings page and still leave the user
  // hunting for the plan section — the dead end it exists to remove.
  it("shows the change-plan dialog when the route's ?plan=change says so", () => {
    render(
      <WorkspaceDetailsCard
        workspace={workspace}
        changePlanOpen
        onChangePlanOpenChange={vi.fn()}
      />,
    );
    expect(screen.getByTestId("change-plan-dialog")).toBeInTheDocument();
  });

  it("renders the created date long-form, or an em dash when unknown", () => {
    // Zone-less input parses as local time, so this holds in any runner TZ.
    const { rerender } = render(
      <WorkspaceDetailsCard
        workspace={{ ...workspace, createdAt: "2026-07-16T00:57:00" }}
        changePlanOpen={false}
        onChangePlanOpenChange={vi.fn()}
      />,
    );
    expect(screen.getByText("July 16, 2026")).toBeInTheDocument();

    rerender(
      <WorkspaceDetailsCard
        workspace={workspace}
        changePlanOpen={false}
        onChangePlanOpenChange={vi.fn()}
      />,
    );
    expect(screen.getByText("—")).toBeInTheDocument();
  });

  it("asks the route to open the dialog when the plan link is clicked", async () => {
    const onChangePlanOpenChange = vi.fn();
    const user = userEvent.setup();
    render(
      <WorkspaceDetailsCard
        workspace={workspace}
        changePlanOpen={false}
        onChangePlanOpenChange={onChangePlanOpenChange}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Change plan" }));

    // The card never holds this state itself — the URL does.
    expect(onChangePlanOpenChange).toHaveBeenCalledWith(true);
  });
});

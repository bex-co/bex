import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { ChangePlanDialog } from "@/features/workspaces/components/change-plan-dialog";
import type { WorkspaceView } from "@/features/workspaces/types";

// The real useChangeWorkspacePlan over a mocked mutation: the dialog's lines
// come from the refusal's code and params, never from its message.
const mutate = vi.hoisted(() => vi.fn());
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate],
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const PRIMARY: WorkspaceView = {
  id: "tea-primary",
  name: "acme",
  slug: "acme",
  plan: "scale",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
  requireGoogleLogin: false,
  organization: {
    id: "org-1",
    name: "Acme",
    primaryWorkspaceId: "tea-primary",
    requireTwoFactor: false,
    requireGoogleLogin: false,
  },
  primary: true,
  privacy: "invite_only",
};

const STANDALONE: WorkspaceView = {
  ...PRIMARY,
  plan: "pro",
  organization: null,
  primary: false,
  privacy: null,
};

/** The inline refusal, found by its title (the plan cards have lists too). */
async function refusal(): Promise<HTMLElement> {
  const title = await screen.findByText("Couldn't change plan");
  return title.closest<HTMLElement>('[data-slot="alert"]')!;
}

function renderDialog(workspace: WorkspaceView) {
  return render(
    <ChangePlanDialog
      workspace={workspace}
      open
      onOpenChange={vi.fn()}
      onChanged={vi.fn()}
    />,
  );
}

beforeEach(() => {
  mutate.mockReset();
});

describe("ChangePlanDialog on an organization's primary (w7/m464)", () => {
  it("warns that a plan below Scale deletes the organization, and only then", async () => {
    const user = userEvent.setup();
    renderDialog(PRIMARY);

    expect(
      screen.queryByTestId("organization-downgrade-warning"),
    ).not.toBeInTheDocument();
    await user.click(screen.getByRole("radio", { name: /Enterprise/ }));
    expect(
      screen.queryByTestId("organization-downgrade-warning"),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("radio", { name: /Pro/ }));
    const warning = screen.getByTestId("organization-downgrade-warning");
    expect(warning).toHaveTextContent(
      "Downgrading below Scale deletes the organization",
    );
    expect(warning).toHaveTextContent(
      "this workspace continues as a standalone workspace",
    );
  });

  it("never warns about an organization on a standalone workspace", async () => {
    const user = userEvent.setup();
    renderDialog(STANDALONE);

    await user.click(screen.getByRole("radio", { name: /Hobby/ }));
    expect(
      screen.queryByTestId("organization-downgrade-warning"),
    ).not.toBeInTheDocument();
  });

  it("lists every unmet precondition of ORGANIZATION_DOWNGRADE_BLOCKED, one line per kind", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("ORGANIZATION_DOWNGRADE_BLOCKED", {
        organizationId: "org-1",
        plan: "pro",
        preconditions: [
          { kind: "secondary_workspaces", items: ["staging", "data"] },
          { kind: "member_roles", items: ["kratos-id-1:billing"] },
          { kind: "invite_roles", items: ["ann@example.com:viewer"] },
          {
            kind: "inbound_ip_rules",
            items: ["workspace", "environment:production", "service:api"],
          },
          // A kind this dashboard has no copy for is left out, not shown raw.
          { kind: "future_kind", items: ["x"] },
        ],
      }),
    );
    const user = userEvent.setup();
    renderDialog(PRIMARY);

    await user.click(screen.getByRole("radio", { name: /Pro/ }));
    await user.click(screen.getByRole("button", { name: "Change Plan" }));

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "tea-primary", plan: "pro" },
    });
    const alert = await refusal();
    expect(alert).toHaveTextContent(
      "The organization can't be deleted yet. Before you downgrade below Scale:",
    );
    expect(alert).not.toHaveTextContent("reworded by the server");
    const lines = within(alert)
      .getAllByRole("listitem")
      .map((item) => item.textContent);
    expect(lines).toEqual([
      "Delete the organization's other workspaces: staging and data",
      "Change these members' roles to Developer or Admin: kratos-id-1 (Billing)",
      "Change these pending invites' roles to Developer or Admin: ann@example.com (Viewer)",
      "Delete inbound IP rules: this workspace, environment production, and service api",
    ]);
  });

  it("names the kind alone when the refusal carries no items", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("ORGANIZATION_DOWNGRADE_BLOCKED", {
        preconditions: [{ kind: "secondary_workspaces", items: null }],
      }),
    );
    const user = userEvent.setup();
    renderDialog(PRIMARY);

    await user.click(screen.getByRole("radio", { name: /Hobby/ }));
    await user.click(screen.getByRole("button", { name: "Change Plan" }));

    const item = within(await refusal()).getByRole("listitem");
    expect(item).toHaveTextContent(
      /^Delete the organization's other workspaces$/,
    );
  });

  it("words ORGANIZATION_PLAN_ON_PRIMARY from its code", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("ORGANIZATION_PLAN_ON_PRIMARY", {
        organizationId: "org-1",
        primaryWorkspaceId: "tea-primary",
      }),
    );
    const user = userEvent.setup();
    renderDialog({ ...PRIMARY, id: "tea-secondary", primary: false });

    await user.click(screen.getByRole("radio", { name: /Enterprise/ }));
    await user.click(screen.getByRole("button", { name: "Change Plan" }));

    const alert = await refusal();
    expect(alert).toHaveTextContent(
      "This workspace's plan is its organization's plan. Change it on the organization's primary workspace.",
    );
    expect(within(alert).queryByRole("listitem")).not.toBeInTheDocument();
  });
});

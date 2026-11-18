import type { ReactElement } from "react";
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { shape } from "@/test/card-shape";
import { WorkspaceDetailsCard } from "@/features/workspaces/components/workspace-details-card";
import { WorkspaceDetailsCardSkeleton } from "@/features/workspaces/components/workspace-details-card-skeleton";
import {
  organizationPlanOwner,
  type WorkspaceView,
} from "@/features/workspaces/types";

const rename = vi.fn();
vi.mock("@/features/workspaces/hooks/use-rename-workspace", () => ({
  useRenameWorkspace: () => ({ rename, busy: false, error: null }),
}));

// The caller's workspaces: a secondary links to its primary only when the
// caller is in it.
const workspaceList = vi.hoisted(() => ({ value: [] as { id: string }[] }));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ refetch: vi.fn(), workspaces: workspaceList.value }),
}));

// Stand in for the real dialog: what matters here is whether the card opens it.
vi.mock("@/features/workspaces/components/change-plan-dialog", () => ({
  ChangePlanDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="change-plan-dialog" /> : null,
}));

const workspace: WorkspaceView = {
  id: "tea-1",
  name: "acme",
  slug: "acme",
  plan: "hobby",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
  requireGoogleLogin: false,
  organization: null,
  primary: false,
  privacy: null,
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

// w5/059: the switcher swaps this card's workspace prop without remounting it,
// so a draft seeded once at mount kept the PREVIOUS workspace's name in the
// input while Plan/ID/Created updated around it. Save sits beside that input,
// so pressing it would have renamed the newly selected workspace to the old
// one's name.
describe("WorkspaceDetailsCard — switching workspaces (w5/059)", () => {
  const other: WorkspaceView = {
    id: "tea-2",
    name: "beta",
    slug: "beta",
    plan: "pro",
    role: "admin",
    createdAt: null,
    requireTwoFactor: false,
    requireGoogleLogin: false,
    organization: null,
    primary: false,
    privacy: null,
  };

  it("re-seeds the name input when pointed at a different workspace", () => {
    const { rerender } = render(
      <WorkspaceDetailsCard
        workspace={workspace}
        changePlanOpen={false}
        onChangePlanOpenChange={vi.fn()}
      />,
    );
    expect(screen.getByDisplayValue("acme")).toBeInTheDocument();

    rerender(
      <WorkspaceDetailsCard
        workspace={other}
        changePlanOpen={false}
        onChangePlanOpenChange={vi.fn()}
      />,
    );
    expect(screen.getByDisplayValue("beta")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("acme")).not.toBeInTheDocument();
  });

  it("does not clobber an in-progress edit of the same workspace", async () => {
    // The rename flow refetches on success, which re-renders this card with the
    // same workspace — a name-keyed reset would wipe whatever is being typed.
    const { rerender } = render(
      <WorkspaceDetailsCard
        workspace={workspace}
        changePlanOpen={false}
        onChangePlanOpenChange={vi.fn()}
      />,
    );
    const input = screen.getByDisplayValue("acme");
    await userEvent.clear(input);
    await userEvent.type(input, "acme-renamed");

    rerender(
      <WorkspaceDetailsCard
        workspace={workspace}
        changePlanOpen={false}
        onChangePlanOpenChange={vi.fn()}
      />,
    );
    expect(screen.getByDisplayValue("acme-renamed")).toBeInTheDocument();
  });

  // w3/m246: the name is Render's freeform display name; the slug is shown
  // read-only next to the id.
  it("renames to a freeform name and shows the slug", async () => {
    rename.mockReset();
    rename.mockResolvedValue(true);
    render(
      <WorkspaceDetailsCard
        workspace={{ ...workspace, slug: "acme-key" }}
        changePlanOpen={false}
        onChangePlanOpenChange={vi.fn()}
      />,
    );
    expect(screen.getByText("acme-key")).toBeInTheDocument();
    const input = screen.getByDisplayValue("acme");
    await userEvent.clear(input);
    await userEvent.type(input, "  Tian Pan's Workspace ");
    expect(input).toHaveValue("  Tian Pan's Workspace ");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(rename).toHaveBeenCalledWith("tea-1", "Tian Pan's Workspace");
  });

  it("refuses a blank name", async () => {
    render(
      <WorkspaceDetailsCard
        workspace={workspace}
        changePlanOpen={false}
        onChangePlanOpenChange={vi.fn()}
      />,
    );
    const input = screen.getByDisplayValue("acme");
    await userEvent.clear(input);
    await userEvent.type(input, "   ");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });
});

// w7/m464 (ADR115 D2): a secondary workspace's plan is its organization's,
// managed on the primary, as on Render: no change control here, and a link to
// the primary's settings.
describe("WorkspaceDetailsCard — an organization's secondary (w7/m464)", () => {
  const secondary: WorkspaceView = {
    ...workspace,
    id: "tea-secondary",
    plan: "scale",
    organization: {
      id: "org-1",
      name: "Acme",
      primaryWorkspaceId: "tea-primary",
      requireTwoFactor: false,
      requireGoogleLogin: false,
    },
    primary: false,
    privacy: "public",
  };

  /** The note links through the router, so it renders under a real one. */
  async function renderUnderRouter(ui: ReactElement) {
    const rootRoute = createRootRoute();
    const route = createRoute({
      getParentRoute: () => rootRoute,
      path: "/",
      component: () => ui,
    });
    const router = createRouter({
      routeTree: rootRoute.addChildren([route]),
      history: createMemoryHistory({ initialEntries: ["/"] }),
      context: { client: {} as never, session: null },
    });
    await router.load();
    return render(<RouterProvider router={router} />);
  }

  it("shows the plan read-only, with a link to the primary's settings", async () => {
    workspaceList.value = [{ id: "tea-primary" }, { id: "tea-secondary" }];
    await renderUnderRouter(
      <WorkspaceDetailsCard
        workspace={secondary}
        changePlanOpen
        onChangePlanOpenChange={vi.fn()}
      />,
    );

    expect(screen.getByText("Scale")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Change plan" }),
    ).not.toBeInTheDocument();
    // Not even `?plan=change` opens a plan editor here.
    expect(screen.queryByTestId("change-plan-dialog")).not.toBeInTheDocument();
    expect(screen.getByTestId("organization-plan-note")).toHaveTextContent(
      "Included in your organization's plan. The organization's primary workspace manages its plan and billing.",
    );
    expect(
      screen.getByRole("link", { name: "Go to the primary workspace" }),
    ).toHaveAttribute("href", "/w/tea-primary/settings");
  });

  it("names the primary without a link when the caller is not in it", async () => {
    workspaceList.value = [{ id: "tea-secondary" }];
    await renderUnderRouter(
      <WorkspaceDetailsCard
        workspace={secondary}
        changePlanOpen={false}
        onChangePlanOpenChange={vi.fn()}
      />,
    );

    expect(screen.getByTestId("organization-plan-note")).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Change plan" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the change-plan control on the organization's primary", () => {
    workspaceList.value = [{ id: "tea-primary" }];
    render(
      <WorkspaceDetailsCard
        workspace={{ ...secondary, id: "tea-primary", primary: true }}
        changePlanOpen
        onChangePlanOpenChange={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Change plan" }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("change-plan-dialog")).toBeInTheDocument();
    expect(
      screen.queryByTestId("organization-plan-note"),
    ).not.toBeInTheDocument();
  });

  // AGENTS.md "Shape parity is mandatory": the pending card reserves the
  // note's own paragraph, with the same copy, so the swap shifts nothing.
  it.each([
    ["in the primary", ["tea-primary", "tea-secondary"]],
    ["not in the primary", ["tea-secondary"]],
  ])("previews the note's paragraph for a caller %s", async (_, ids) => {
    workspaceList.value = ids.map((id) => ({ id }));
    const { container: pending } = render(
      <WorkspaceDetailsCardSkeleton
        planOwner={organizationPlanOwner(secondary, workspaceList.value)}
      />,
    );
    const { container: ready } = await renderUnderRouter(
      <WorkspaceDetailsCard
        workspace={secondary}
        changePlanOpen={false}
        onChangePlanOpenChange={vi.fn()}
      />,
    );
    const pendingNote = pending.querySelector<HTMLElement>(
      '[data-skeleton-region="organization-plan-note"]',
    )!;
    const readyNote = ready.querySelector<HTMLElement>(
      '[data-testid="organization-plan-note"]',
    )!;
    expect(pendingNote.className).toBe(readyNote.className);
    // The same words, reserved invisibly: the bar wraps as the copy does.
    expect(pendingNote.textContent).toBe(readyNote.textContent);
    expect(pendingNote.querySelector("a")).toBeNull();
    // The note is the content's last child in both, after the same grid.
    const pendingContent = pendingNote.parentElement!;
    const readyContent = readyNote.parentElement!;
    expect(pendingContent.lastElementChild).toBe(pendingNote);
    expect(readyContent.lastElementChild).toBe(readyNote);
    expect(shape(pendingContent.querySelector("dl")!).split("[")[0]).toBe(
      shape(readyContent.querySelector("dl")!).split("[")[0],
    );
  });

  it("draws no note for a standalone workspace's pending card", () => {
    const { container } = render(
      <WorkspaceDetailsCardSkeleton
        planOwner={organizationPlanOwner(workspace, [])}
      />,
    );
    expect(
      container.querySelector(
        '[data-skeleton-region="organization-plan-note"]',
      ),
    ).toBeNull();
  });
});

import type { ReactElement } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { shape } from "@/test/card-shape";
import { WorkspaceTwoFactorCard } from "@/features/workspaces/components/workspace-two-factor-card";
import { WorkspaceTwoFactorCardSkeleton } from "@/features/workspaces/components/workspace-two-factor-card-skeleton";
import type { WorkspaceView } from "@/features/workspaces/types";

const mutate = vi.hoisted(() => vi.fn());
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate, { loading: false }],
}));

const toastSuccess = vi.hoisted(() => vi.fn());
const changePlan = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: toastSuccess, error: vi.fn() },
}));

// `role` is "admin" for every membership the workspace list returns, whatever
// the member's real role (a live developer's workspace says "admin"), so the
// card must decide from the capability check, never from this field.
const WORKSPACE: WorkspaceView = {
  id: "tea-acme",
  name: "Acme",
  slug: "acme",
  plan: "pro",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
  organization: null,
  primary: false,
  privacy: null
};

/** The card links to account security, so it renders under a real router. */
async function renderCard(ui: ReactElement) {
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

function asAdmin() {
  vi.mocked(useCapabilities).mockReturnValue(
    mockCapabilities({ role: "ADMIN", canManage: true }),
  );
}

function asRole(role: string) {
  vi.mocked(useCapabilities).mockReturnValue(
    mockCapabilities({ role: role.toUpperCase(), canManage: false }),
  );
}

beforeEach(() => {
  asAdmin();
  mutate.mockReset();
  mutate.mockResolvedValue({ data: {} });
  toastSuccess.mockReset();
  changePlan.mockReset();
});

describe("WorkspaceTwoFactorCard — admin (w22/m402)", () => {
  it("explains the enforcement in Render's terms", async () => {
    await renderCard(
      <WorkspaceTwoFactorCard
        workspace={WORKSPACE}
        onChangePlan={changePlan}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Two-factor authentication" }),
    ).toBeInTheDocument();
    const description = document.querySelector(
      '[data-slot="card-description"]',
    );
    expect(description).toHaveTextContent(
      "Members without two-factor authentication (2FA) can't access this workspace's resources or settings until they enable it.",
    );
    expect(description).toHaveTextContent(
      "Their API keys and SSH keys are blocked until then too.",
    );
    expect(description).toHaveTextContent(
      "Only workspace admins can change this setting.",
    );
  });

  it("turns the requirement on with the switch and says so", async () => {
    const user = userEvent.setup();
    await renderCard(
      <WorkspaceTwoFactorCard
        workspace={WORKSPACE}
        onChangePlan={changePlan}
      />,
    );

    const toggle = screen.getByRole("switch", {
      name: "Enforce two-factor authentication (2FA)",
    });
    expect(toggle).not.toBeChecked();
    await user.click(toggle);

    expect(mutate).toHaveBeenCalledWith({
      variables: { workspaceId: "tea-acme", enabled: true },
    });
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith(
        "Two-factor authentication is now required in Acme",
      ),
    );
  });

  it("turns it off from the on state", async () => {
    const user = userEvent.setup();
    await renderCard(
      <WorkspaceTwoFactorCard
        workspace={{ ...WORKSPACE, requireTwoFactor: true, organization: null, primary: false, privacy: null }}
        onChangePlan={changePlan}
      />,
    );

    const toggle = screen.getByRole("switch");
    expect(toggle).toBeChecked();
    await user.click(toggle);

    expect(mutate).toHaveBeenCalledWith({
      variables: { workspaceId: "tea-acme", enabled: false },
    });
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith(
        "Two-factor authentication is no longer required in Acme",
      ),
    );
  });

  // The refusals are decided by their codes: the server's wording
  // ("reworded by the server") is never what the admin reads.
  it("shows the admin-not-enrolled refusal with a way to enroll", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("WORKSPACE_2FA_ADMIN_NOT_ENROLLED"),
    );
    const user = userEvent.setup();
    await renderCard(
      <WorkspaceTwoFactorCard
        workspace={WORKSPACE}
        onChangePlan={changePlan}
      />,
    );

    await user.click(screen.getByRole("switch"));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Couldn't change the two-factor setting");
    expect(alert).toHaveTextContent(
      "Turn on two-factor authentication for your own account before requiring it in this workspace",
    );
    expect(alert).not.toHaveTextContent("reworded by the server");
    expect(
      screen.getByRole("link", { name: "Set up two-factor authentication" }),
    ).toHaveAttribute("href", "/settings#security");
    // The write did not happen, so the switch still shows the stored value.
    expect(screen.getByRole("switch")).not.toBeChecked();
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("shows the unavailable refusal without an enroll link", async () => {
    mutate.mockRejectedValue(codedGraphQLError("WORKSPACE_2FA_UNAVAILABLE"));
    const user = userEvent.setup();
    await renderCard(
      <WorkspaceTwoFactorCard
        workspace={WORKSPACE}
        onChangePlan={changePlan}
      />,
    );

    await user.click(screen.getByRole("switch"));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Two-factor enforcement is unavailable on this installation, so it can't be turned on.",
    );
    expect(
      screen.queryByRole("link", { name: "Set up two-factor authentication" }),
    ).not.toBeInTheDocument();
  });

  it("falls back to the generic message for a failure with no answer", async () => {
    mutate.mockRejectedValue(new Error("Failed to fetch"));
    const user = userEvent.setup();
    await renderCard(
      <WorkspaceTwoFactorCard
        workspace={WORKSPACE}
        onChangePlan={changePlan}
      />,
    );

    await user.click(screen.getByRole("switch"));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Couldn't change the two-factor setting. Try again.",
    );
  });

  it("clears a refusal when the next attempt starts", async () => {
    mutate.mockRejectedValueOnce(
      codedGraphQLError("WORKSPACE_2FA_UNAVAILABLE"),
    );
    const user = userEvent.setup();
    await renderCard(
      <WorkspaceTwoFactorCard
        workspace={WORKSPACE}
        onChangePlan={changePlan}
      />,
    );

    await user.click(screen.getByRole("switch"));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    await user.click(screen.getByRole("switch"));

    await waitFor(() =>
      expect(screen.queryByRole("alert")).not.toBeInTheDocument(),
    );
  });
});

// Render's plan table keeps "Enforce 2FA & Google login" to Pro and higher.
describe("WorkspaceTwoFactorCard — below Pro (w7/070)", () => {
  const HOBBY: WorkspaceView = { ...WORKSPACE, plan: "hobby" };
  const PLAN_NOTICE =
    "Enforcing two-factor authentication requires the Pro plan or higher.";

  it("keeps an admin's switch off and offers the plan change", async () => {
    const user = userEvent.setup();
    await renderCard(
      <WorkspaceTwoFactorCard workspace={HOBBY} onChangePlan={changePlan} />,
    );

    const toggle = screen.getByRole("switch", {
      name: "Enforce two-factor authentication (2FA)",
    });
    expect(toggle).toBeDisabled();
    expect(screen.getByText(new RegExp(PLAN_NOTICE))).toBeInTheDocument();
    await user.click(toggle);
    expect(mutate).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Change plan" }));
    expect(changePlan).toHaveBeenCalledOnce();
  });

  it("lets an admin turn off a requirement the workspace already has", async () => {
    const user = userEvent.setup();
    await renderCard(
      <WorkspaceTwoFactorCard
        workspace={{ ...HOBBY, requireTwoFactor: true, organization: null, primary: false, privacy: null }}
        onChangePlan={changePlan}
      />,
    );

    const toggle = screen.getByRole("switch");
    expect(toggle).toBeChecked();
    expect(toggle).toBeEnabled();
    expect(
      screen.getByText(/keeps it until an admin turns it off/),
    ).toBeInTheDocument();
    await user.click(toggle);
    expect(mutate).toHaveBeenCalledWith({
      variables: { workspaceId: "tea-acme", enabled: false },
    });
  });

  it("tells a member why, without the plan change", async () => {
    asRole("developer");
    await renderCard(
      <WorkspaceTwoFactorCard workspace={HOBBY} onChangePlan={changePlan} />,
    );

    expect(screen.getByText(new RegExp(PLAN_NOTICE))).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Change plan" }),
    ).not.toBeInTheDocument();
  });

  it("says nothing about the plan on Pro", async () => {
    await renderCard(
      <WorkspaceTwoFactorCard
        workspace={WORKSPACE}
        onChangePlan={changePlan}
      />,
    );

    expect(screen.queryByText(new RegExp(PLAN_NOTICE))).not.toBeInTheDocument();
    expect(screen.getByRole("switch")).toBeEnabled();
  });

  it("reads the server's plan refusal by its code", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("PLAN_FEATURE_REQUIRED", {
        feature: "twoFactorEnforcement",
        plan: "hobby",
        requiredPlan: "pro",
      }),
    );
    const user = userEvent.setup();
    await renderCard(
      <WorkspaceTwoFactorCard
        workspace={WORKSPACE}
        onChangePlan={changePlan}
      />,
    );

    await user.click(screen.getByRole("switch"));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(PLAN_NOTICE);
    expect(alert).not.toHaveTextContent("reworded by the server");
  });
});

describe("WorkspaceTwoFactorCard — everyone else (w22/m402)", () => {
  it.each(["developer", "viewer", "billing", "contributor"])(
    "shows a %s the state without a control",
    async (role) => {
      asRole(role);
      await renderCard(
        <WorkspaceTwoFactorCard
          workspace={WORKSPACE}
          onChangePlan={changePlan}
        />,
      );

      expect(screen.queryByRole("switch")).not.toBeInTheDocument();
      expect(screen.getByText("Not required")).toBeInTheDocument();
      expect(
        screen.getByText(/Only workspace admins can change this setting/),
      ).toBeInTheDocument();
    },
  );

  it("says when the workspace requires it", async () => {
    asRole("developer");
    await renderCard(
      <WorkspaceTwoFactorCard
        workspace={{ ...WORKSPACE, requireTwoFactor: true, organization: null, primary: false, privacy: null }}
        onChangePlan={changePlan}
      />,
    );

    expect(screen.getByText("Required")).toBeInTheDocument();
    expect(screen.queryByRole("switch")).not.toBeInTheDocument();
    expect(mutate).not.toHaveBeenCalled();
  });

  it("offers no control and no verdict while the capability check is in flight", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: null, canManage: false, loading: true }),
    );
    const { container } = await renderCard(
      <WorkspaceTwoFactorCard
        workspace={WORKSPACE}
        onChangePlan={changePlan}
      />,
    );

    expect(screen.queryByRole("switch")).not.toBeInTheDocument();
    expect(screen.queryByText("Not required")).not.toBeInTheDocument();
    expect(
      container.querySelector('[data-skeleton-region="two-factor-control"]'),
    ).toHaveClass("h-5", "w-9");
  });
});

// AGENTS.md "Shape parity is mandatory": the preview is the ready card's own
// anatomy with only the role-dependent control held as a placeholder.
describe("WorkspaceTwoFactorCardSkeleton matches the card (w22/m402)", () => {
  async function renderReady(workspace: WorkspaceView) {
    const { container } = await renderCard(
      <WorkspaceTwoFactorCard
        workspace={workspace}
        onChangePlan={changePlan}
      />,
    );
    return container.querySelector<HTMLElement>('[data-slot="card"]')!;
  }

  it.each([
    ["an admin", asAdmin],
    ["a non-admin", () => asRole("developer")],
  ])("has the same card, header and row as %s sees", async (_who, as) => {
    as();
    const { container } = render(<WorkspaceTwoFactorCardSkeleton />);
    const pending = container.querySelector<HTMLElement>('[data-slot="card"]')!;
    const ready = await renderReady(WORKSPACE);

    expect(pending.className).toBe(ready.className);
    expect(shape(pending.querySelector('[data-slot="card-header"]')!)).toBe(
      shape(ready.querySelector('[data-slot="card-header"]')!),
    );
    const pendingContent = pending.querySelector('[data-slot="card-content"]')!;
    const readyContent = ready.querySelector('[data-slot="card-content"]')!;
    expect(pendingContent.className).toBe(readyContent.className);
    const pendingRow = pendingContent.firstElementChild!;
    const readyRow = readyContent.firstElementChild!;
    expect(pendingRow.className).toBe(readyRow.className);
    // The label is static copy, so it is the same element with the same text.
    expect(shape(pendingRow.firstElementChild!)).toBe(
      shape(readyRow.firstElementChild!),
    );
  });

  it("holds the switch's own box, and offers no control before its data exists", async () => {
    const { container } = render(<WorkspaceTwoFactorCardSkeleton />);
    const placeholder = container.querySelector<HTMLElement>(
      '[data-skeleton-region="two-factor-control"]',
    )!;
    const ready = await renderReady(WORKSPACE);
    const toggle = ready.querySelector<HTMLElement>('[role="switch"]')!;

    for (const box of ["h-5", "w-9", "shrink-0", "rounded-full"]) {
      expect(placeholder).toHaveClass(box);
      expect(toggle).toHaveClass(box);
    }
    expect(container.querySelector("button, [role='switch'], a")).toBeNull();
  });
});

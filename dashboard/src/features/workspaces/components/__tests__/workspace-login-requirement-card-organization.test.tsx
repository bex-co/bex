import type { ReactElement } from "react";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { shape } from "@/test/card-shape";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useOrganizations } from "@/features/organizations/hooks/use-organizations";
import { organization } from "@/features/organizations/__tests__/org-fixtures";
import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";
import { WorkspaceLoginRequirementCard } from "@/features/workspaces/components/workspace-login-requirement-card";
import { WorkspaceLoginRequirementCardSkeleton } from "@/features/workspaces/components/workspace-login-requirement-card-skeleton";
import type { OrganizationRole } from "@/features/organizations/types";
import type { WorkspaceView } from "@/features/workspaces/types";

const mutate = vi.hoisted(() => vi.fn());
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate, { loading: false }],
}));

// w7/m466: a workspace in an org that requires 2FA or Google login in all of
// its workspaces shows its own switch on and locked, naming the org.
function workspace(
  settings: Partial<Pick<WorkspaceView, "requireTwoFactor">> & {
    org?: { requireTwoFactor?: boolean; requireGoogleLogin?: boolean };
  } = {},
): WorkspaceView {
  return {
    id: "tea-web",
    name: "acme-web",
    slug: "acme-web",
    plan: "scale",
    role: "admin",
    createdAt: null,
    requireTwoFactor: settings.requireTwoFactor ?? false,
    requireGoogleLogin: false,
    organization: {
      id: "org-1",
      name: "Acme",
      primaryWorkspaceId: "tea-hq",
      requireTwoFactor: settings.org?.requireTwoFactor ?? false,
      requireGoogleLogin: settings.org?.requireGoogleLogin ?? false,
    },
    primary: false,
    privacy: "invite_only",
  };
}

const SWITCH_LABELS: Record<WorkspaceLoginRequirement, string> = {
  twoFactor: "Enforce two-factor authentication (2FA)",
  googleLogin: "Enforce Google login",
};

/** The note links to the org's Settings, so the card renders under a router
 *  that has that route. */
async function renderUnderRouter(ui: ReactElement) {
  const rootRoute = createRootRoute();
  const routes = ["/", "/org/$orgId/settings"].map((path) =>
    createRoute({
      getParentRoute: () => rootRoute,
      path,
      component: () => ui,
    }),
  );
  const router = createRouter({
    routeTree: rootRoute.addChildren(routes),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

function renderCard(
  requirement: WorkspaceLoginRequirement,
  view: WorkspaceView,
) {
  return renderUnderRouter(
    <WorkspaceLoginRequirementCard
      requirement={requirement}
      workspace={view}
      onChangePlan={vi.fn()}
    />,
  );
}

function asAdmin(admin: boolean) {
  vi.mocked(useCapabilities).mockReturnValue(
    mockCapabilities({
      role: admin ? "ADMIN" : "DEVELOPER",
      canManage: admin,
    }),
  );
}

/** The caller's role in Acme, as the switcher's org list reports it. */
function inOrganizationAs(role: OrganizationRole | null) {
  vi.mocked(useOrganizations).mockReturnValue({
    organizations: role ? [organization(role)] : [],
  });
}

beforeEach(() => {
  asAdmin(true);
  inOrganizationAs("MEMBER");
  mutate.mockReset();
});

describe("WorkspaceLoginRequirementCard in an org that requires it (w7/m466)", () => {
  it.each(["twoFactor", "googleLogin"] as const)(
    "holds an admin's %s switch on and locked, whatever the workspace's own setting",
    async (requirement) => {
      await renderCard(
        requirement,
        workspace({
          org:
            requirement === "twoFactor"
              ? { requireTwoFactor: true }
              : { requireGoogleLogin: true },
        }),
      );

      const toggle = screen.getByRole("switch", {
        name: SWITCH_LABELS[requirement],
      });
      expect(toggle).toBeChecked();
      expect(toggle).toBeDisabled();
      expect(
        screen.getByTestId("organization-requirement-note"),
      ).toHaveTextContent("Required by Acme in all of its workspaces.");
      // A Member of the org reads why, without the way to the org's setting.
      expect(
        screen.queryByRole("link", {
          name: "Change it in the organization's settings",
        }),
      ).toBeNull();
      expect(mutate).not.toHaveBeenCalled();
    },
  );

  it("gives an Owner of the org the way to its Settings page", async () => {
    inOrganizationAs("OWNER");
    await renderCard(
      "twoFactor",
      workspace({ org: { requireTwoFactor: true } }),
    );

    expect(
      screen.getByRole("link", {
        name: "Change it in the organization's settings",
      }),
    ).toHaveAttribute("href", "/org/org-1/settings");
  });

  it("shows a member who is not an admin the state as required, with the org's note", async () => {
    asAdmin(false);
    await renderCard(
      "googleLogin",
      workspace({ org: { requireGoogleLogin: true } }),
    );

    expect(screen.queryByRole("switch")).toBeNull();
    expect(screen.getByText("Required")).toBeInTheDocument();
    expect(
      screen.getByTestId("organization-requirement-note"),
    ).toHaveTextContent("Required by Acme in all of its workspaces.");
  });

  // Only the requirement the org holds is locked; turning the org's setting
  // off gives the workspace its own setting back as it was.
  it("leaves the other requirement, and an org that requires nothing, to the workspace", async () => {
    await renderCard(
      "googleLogin",
      workspace({ org: { requireTwoFactor: true } }),
    );
    let toggle = screen.getByRole("switch", { name: "Enforce Google login" });
    expect(toggle).not.toBeChecked();
    expect(toggle).toBeEnabled();
    expect(screen.queryByTestId("organization-requirement-note")).toBeNull();

    document.body.innerHTML = "";
    await renderCard("twoFactor", workspace({ requireTwoFactor: false }));
    toggle = screen.getByRole("switch", {
      name: "Enforce two-factor authentication (2FA)",
    });
    expect(toggle).not.toBeChecked();
    expect(toggle).toBeEnabled();
    expect(screen.queryByTestId("organization-requirement-note")).toBeNull();
  });
});

// AGENTS.md "Shape parity is mandatory": a workspace already known to be in
// an enforcing org settles to the card with the org's note, so the preview
// reserves the note's box (the Owner's link included) without its text.
describe("WorkspaceLoginRequirementCardSkeleton in an org that requires it (w7/m466)", () => {
  it.each(["OWNER", "MEMBER"] as const)(
    "reserves the org's note as an org %s sees it",
    async (role) => {
      inOrganizationAs(role);
      const locked = workspace({ org: { requireTwoFactor: true } });
      const { container } = render(
        <WorkspaceLoginRequirementCardSkeleton
          requirement="twoFactor"
          organization={locked.organization}
        />,
      );
      const pending = container.querySelector(
        '[data-skeleton-region="organization-requirement-note"]',
      )!;
      const { container: readyContainer } = await renderCard(
        "twoFactor",
        locked,
      );
      const ready = readyContainer.querySelector(
        '[data-testid="organization-requirement-note"]',
      )!;

      expect(pending.className).toBe(ready.className);
      // The copy is laid out invisibly under a bar, so it wraps as the
      // note will, link included, but nothing in it can be read or used.
      const reserved = pending.querySelector("span.invisible")!;
      expect(reserved.textContent).toBe(ready.textContent);
      expect(pending.querySelector("a, button")).toBeNull();
      expect(shape(container.querySelector('[data-slot="card-header"]')!)).toBe(
        shape(readyContainer.querySelector('[data-slot="card-header"]')!),
      );
    },
  );

  it("draws no note for a workspace whose org requires nothing", () => {
    const { container } = render(
      <WorkspaceLoginRequirementCardSkeleton
        requirement="twoFactor"
        organization={
          workspace({ org: { requireGoogleLogin: true } }).organization
        }
      />,
    );
    expect(
      container.querySelector(
        '[data-skeleton-region="organization-requirement-note"]',
      ),
    ).toBeNull();
  });
});

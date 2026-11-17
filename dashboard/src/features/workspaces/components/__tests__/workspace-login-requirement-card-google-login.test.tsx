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
import type { DocumentNode, OperationDefinitionNode } from "graphql";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { shape } from "@/test/card-shape";
import { WorkspaceLoginRequirementCard } from "@/features/workspaces/components/workspace-login-requirement-card";
import { WorkspaceLoginRequirementCardSkeleton } from "@/features/workspaces/components/workspace-login-requirement-card-skeleton";
import type { WorkspaceView } from "@/features/workspaces/types";

const mutate = vi.hoisted(() => vi.fn());
const mutationDocuments = vi.hoisted(() => [] as DocumentNode[]);
vi.mock("@apollo/client/react", () => ({
  useMutation: (document: DocumentNode) => {
    mutationDocuments.push(document);
    return [mutate, { loading: false }];
  },
}));

const startGoogleSignIn = vi.hoisted(() => vi.fn());
vi.mock("@/features/auth/lib/google-sign-in", () => ({ startGoogleSignIn }));

const toastSuccess = vi.hoisted(() => vi.fn());
const changePlan = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: toastSuccess, error: vi.fn() },
}));

// `role` is "admin" for every membership the workspace list returns, so the
// card decides from the capability check, as its 2FA sibling does.
const WORKSPACE: WorkspaceView = {
  id: "tea-acme",
  name: "Acme",
  slug: "acme",
  plan: "pro",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
  requireGoogleLogin: false,
  organization: null,
  primary: false,
  privacy: null,
};

const LOCK_OUT =
  "Sign in with the Google account of your bex email address before requiring Google login in this workspace, or you would lock yourself out.";

function GoogleLoginCard({ workspace }: { workspace: WorkspaceView }) {
  return (
    <WorkspaceLoginRequirementCard
      requirement="googleLogin"
      workspace={workspace}
      onChangePlan={changePlan}
    />
  );
}

/** The card's 2FA fix is a router link, so it renders under a real router. */
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

function operationName(document: DocumentNode): string | undefined {
  return (document.definitions[0] as OperationDefinitionNode).name?.value;
}

beforeEach(() => {
  asAdmin();
  mutate.mockReset();
  mutate.mockResolvedValue({ data: {} });
  mutationDocuments.length = 0;
  startGoogleSignIn.mockReset();
  startGoogleSignIn.mockResolvedValue(true);
  toastSuccess.mockReset();
  changePlan.mockReset();
});

describe("WorkspaceLoginRequirementCard — Google login, admin (w7/m465)", () => {
  it("explains the enforcement in Render's terms", async () => {
    await renderCard(<GoogleLoginCard workspace={WORKSPACE} />);

    expect(
      screen.getByRole("heading", { name: "Google login" }),
    ).toBeInTheDocument();
    const description = document.querySelector(
      '[data-slot="card-description"]',
    );
    expect(description).toHaveTextContent(
      "Members who don't sign in with the Google account of their bex email address can't access this workspace's resources or settings.",
    );
    expect(description).toHaveTextContent(
      "API keys and SSH keys created while signed in another way are blocked too.",
    );
    expect(description).toHaveTextContent(
      "Only workspace admins can change this setting.",
    );
  });

  it("turns the requirement on with its own mutation and says so", async () => {
    const user = userEvent.setup();
    await renderCard(<GoogleLoginCard workspace={WORKSPACE} />);

    const toggle = screen.getByRole("switch", { name: "Enforce Google login" });
    expect(toggle).toHaveAttribute("id", "workspace-google-login");
    expect(toggle).not.toBeChecked();
    await user.click(toggle);

    expect(mutationDocuments.map(operationName)).toContain(
      "SetWorkspaceRequireGoogleLogin",
    );
    expect(mutationDocuments.map(operationName)).not.toContain(
      "SetWorkspaceRequireTwoFactor",
    );
    expect(mutate).toHaveBeenCalledWith({
      variables: { workspaceId: "tea-acme", enabled: true },
    });
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith(
        "Google login is now required in Acme",
      ),
    );
  });

  it("turns it off from the on state", async () => {
    const user = userEvent.setup();
    await renderCard(
      <GoogleLoginCard
        workspace={{ ...WORKSPACE, requireGoogleLogin: true }}
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
        "Google login is no longer required in Acme",
      ),
    );
  });

  // The two requirements are separate settings: one being on says nothing
  // about the other.
  it("reads its own setting, not two-factor's", async () => {
    await renderCard(
      <GoogleLoginCard workspace={{ ...WORKSPACE, requireTwoFactor: true }} />,
    );
    expect(screen.getByRole("switch")).not.toBeChecked();
  });

  // The lock-out guard is decided by its code, never the server's wording,
  // and its fix is signing this session in with Google, back to this page.
  it("shows the lock-out refusal with a way to sign in with Google", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN"),
    );
    const user = userEvent.setup();
    await renderCard(<GoogleLoginCard workspace={WORKSPACE} />);

    await user.click(screen.getByRole("switch"));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Couldn't change the Google login setting");
    expect(alert).toHaveTextContent(LOCK_OUT);
    expect(alert).not.toHaveTextContent("reworded by the server");
    // The write did not happen, so the switch still shows the stored value.
    expect(screen.getByRole("switch")).not.toBeChecked();
    expect(toastSuccess).not.toHaveBeenCalled();
    // No 2FA enrollment link: that is the other requirement's fix.
    expect(
      screen.queryByRole("link", { name: "Set up two-factor authentication" }),
    ).not.toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Sign in with Google" }),
    );
    expect(startGoogleSignIn).toHaveBeenCalledWith(window.location.href);
  });

  it("says so when the Google sign-in does not start, and lets the admin retry", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN"),
    );
    startGoogleSignIn.mockRejectedValueOnce(new Error("no oidc provider"));
    const user = userEvent.setup();
    await renderCard(<GoogleLoginCard workspace={WORKSPACE} />);

    await user.click(screen.getByRole("switch"));
    await user.click(
      await screen.findByRole("button", { name: "Sign in with Google" }),
    );

    expect(
      await screen.findByText(
        "Couldn't start signing in with Google. Try again.",
      ),
    ).toBeInTheDocument();
    const retry = screen.getByRole("button", { name: "Sign in with Google" });
    expect(retry).toBeEnabled();
    await user.click(retry);
    expect(startGoogleSignIn).toHaveBeenCalledTimes(2);
  });

  it("shows the unavailable refusal without a sign-in", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("WORKSPACE_GOOGLE_LOGIN_UNAVAILABLE"),
    );
    const user = userEvent.setup();
    await renderCard(<GoogleLoginCard workspace={WORKSPACE} />);

    await user.click(screen.getByRole("switch"));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Google login enforcement is unavailable on this installation, so it can't be turned on.",
    );
    expect(
      screen.queryByRole("button", { name: "Sign in with Google" }),
    ).not.toBeInTheDocument();
  });

  // Each requirement reads only its own codes: a 2FA refusal is not one of
  // this card's, so it gets the generic line, not the 2FA copy.
  it("falls back to the generic message for a code that is not its own", async () => {
    mutate.mockRejectedValue(new Error("Failed to fetch"));
    const user = userEvent.setup();
    await renderCard(<GoogleLoginCard workspace={WORKSPACE} />);

    await user.click(screen.getByRole("switch"));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Couldn't change the Google login setting. Try again.",
    );
  });
});

// Render's plan table keeps "Enforce 2FA & Google login" to Pro and higher.
describe("WorkspaceLoginRequirementCard — Google login below Pro (w7/m465)", () => {
  const HOBBY: WorkspaceView = { ...WORKSPACE, plan: "hobby" };
  const PLAN_NOTICE = "Enforcing Google login requires the Pro plan or higher.";

  it("keeps an admin's switch off and offers the plan change", async () => {
    const user = userEvent.setup();
    await renderCard(<GoogleLoginCard workspace={HOBBY} />);

    const toggle = screen.getByRole("switch", { name: "Enforce Google login" });
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
      <GoogleLoginCard workspace={{ ...HOBBY, requireGoogleLogin: true }} />,
    );

    const toggle = screen.getByRole("switch");
    expect(toggle).toBeChecked();
    expect(toggle).toBeEnabled();
    await user.click(toggle);
    expect(mutate).toHaveBeenCalledWith({
      variables: { workspaceId: "tea-acme", enabled: false },
    });
  });

  it("tells a member why, without the plan change", async () => {
    asRole("developer");
    await renderCard(<GoogleLoginCard workspace={HOBBY} />);

    expect(screen.getByText(new RegExp(PLAN_NOTICE))).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Change plan" }),
    ).not.toBeInTheDocument();
  });

  it("reads the server's plan refusal by its code", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("PLAN_FEATURE_REQUIRED", {
        feature: "googleLoginEnforcement",
        plan: "hobby",
        requiredPlan: "pro",
      }),
    );
    const user = userEvent.setup();
    await renderCard(<GoogleLoginCard workspace={WORKSPACE} />);

    await user.click(screen.getByRole("switch"));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(PLAN_NOTICE);
    expect(alert).not.toHaveTextContent("reworded by the server");
  });
});

describe("WorkspaceLoginRequirementCard — Google login, everyone else (w7/m465)", () => {
  it.each(["developer", "viewer", "billing", "contributor"])(
    "shows a %s the state without a control",
    async (role) => {
      asRole(role);
      await renderCard(<GoogleLoginCard workspace={WORKSPACE} />);

      expect(screen.queryByRole("switch")).not.toBeInTheDocument();
      expect(screen.getByText("Not required")).toBeInTheDocument();
    },
  );

  it("says when the workspace requires it", async () => {
    asRole("developer");
    await renderCard(
      <GoogleLoginCard
        workspace={{ ...WORKSPACE, requireGoogleLogin: true }}
      />,
    );

    expect(screen.getByText("Required")).toBeInTheDocument();
    expect(mutate).not.toHaveBeenCalled();
  });

  it("offers no control and no verdict while the capability check is in flight", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: null, canManage: false, loading: true }),
    );
    const { container } = await renderCard(
      <GoogleLoginCard workspace={WORKSPACE} />,
    );

    expect(screen.queryByRole("switch")).not.toBeInTheDocument();
    expect(screen.queryByText("Not required")).not.toBeInTheDocument();
    expect(
      container.querySelector('[data-skeleton-region="google-login-control"]'),
    ).toHaveClass("h-5", "w-9");
  });
});

// AGENTS.md "Shape parity is mandatory": the preview is the ready card's own
// anatomy with only the role-dependent control held as a placeholder. Every
// class compared here is the same at every width, and the row wraps the same
// way in both, so desktop and narrow-mobile widths match.
describe("WorkspaceLoginRequirementCardSkeleton matches the Google login card (w7/m465)", () => {
  async function renderReady() {
    const { container } = await renderCard(
      <GoogleLoginCard workspace={WORKSPACE} />,
    );
    return container.querySelector<HTMLElement>('[data-slot="card"]')!;
  }

  it.each([
    ["an admin", asAdmin],
    ["a non-admin", () => asRole("developer")],
  ])("has the same card, header and row as %s sees", async (_who, as) => {
    as();
    const { container } = render(
      <WorkspaceLoginRequirementCardSkeleton requirement="googleLogin" />,
    );
    const pending = container.querySelector<HTMLElement>('[data-slot="card"]')!;
    expect(pending).toHaveAttribute(
      "data-testid",
      "workspace-google-login-skeleton",
    );
    const ready = await renderReady();

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

  // A workspace known to be below Pro settles to the card with its notice, so
  // the preview reserves the same paragraph, without the role-gated link.
  it("previews the plan notice of a workspace known to be below Pro", async () => {
    asRole("developer");
    const { container } = render(
      <WorkspaceLoginRequirementCardSkeleton
        requirement="googleLogin"
        belowPro
      />,
    );
    const pendingNotice = container.querySelector(
      '[data-slot="card-content"] > p',
    )!;
    const { container: readyContainer } = await renderCard(
      <GoogleLoginCard workspace={{ ...WORKSPACE, plan: "hobby" }} />,
    );
    const readyNotice = readyContainer.querySelector(
      '[data-slot="card-content"] > p',
    )!;

    expect(shape(pendingNotice)).toBe(shape(readyNotice));
    expect(pendingNotice).toHaveTextContent(
      "Enforcing Google login requires the Pro plan or higher.",
    );
    expect(container.querySelector("button, a")).toBeNull();
    // On Pro and above there is no notice to preview.
    const { container: pro } = render(
      <WorkspaceLoginRequirementCardSkeleton requirement="googleLogin" />,
    );
    expect(pro.querySelector('[data-slot="card-content"] > p')).toBeNull();
  });

  it("holds the switch's own box, and offers no control before its data exists", async () => {
    const { container } = render(
      <WorkspaceLoginRequirementCardSkeleton requirement="googleLogin" />,
    );
    const placeholder = container.querySelector<HTMLElement>(
      '[data-skeleton-region="google-login-control"]',
    )!;
    const ready = await renderReady();
    const toggle = ready.querySelector<HTMLElement>('[role="switch"]')!;

    for (const box of ["h-5", "w-9", "shrink-0", "rounded-full"]) {
      expect(placeholder).toHaveClass(box);
      expect(toggle).toHaveClass(box);
    }
    expect(container.querySelector("button, [role='switch'], a")).toBeNull();
  });
});

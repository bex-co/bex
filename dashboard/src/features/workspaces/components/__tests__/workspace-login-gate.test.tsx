import { useState, type ReactNode } from "react";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { WorkspaceContext } from "@/features/workspaces/context/context";
import { WorkspaceLoginGate } from "@/features/workspaces/components/workspace-login-gate";
import { WorkspaceTwoFactorBlockedSkeleton } from "@/features/workspaces/components/workspace-two-factor-blocked-skeleton";
import { WorkspaceGoogleLoginBlockedSkeleton } from "@/features/workspaces/components/workspace-google-login-blocked-skeleton";
import { shape } from "@/test/card-shape";
import {
  clearWorkspaceLoginBlocks,
  reportWorkspaceLoginRequired,
  workspaceLoginBlock,
} from "@/features/workspaces/lib/workspace-login-block";
import type { WorkspaceView } from "@/features/workspaces/types";

const leaveMutate = vi.hoisted(() => vi.fn());
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [leaveMutate],
}));

const startGoogleSignIn = vi.hoisted(() => vi.fn());
vi.mock("@/features/auth/lib/google-sign-in", () => ({ startGoogleSignIn }));

function workspace(id: string, name: string): WorkspaceView {
  return {
    id,
    name,
    slug: name.toLowerCase(),
    plan: "pro",
    role: "developer",
    createdAt: null,
    requireTwoFactor: true,
    requireGoogleLogin: false,
    organization: null,
    primary: false,
    privacy: null,
  };
}

const ACME = workspace("tea-acme", "Acme");
const BETA = workspace("tea-beta", "Beta");

const refetch = vi.fn();
const selections: string[] = [];

/** WorkspaceProvider's contract with a selection the test controls. */
function Workspaces({
  initial,
  list,
  children,
}: {
  initial: string | null;
  list: WorkspaceView[];
  children: ReactNode;
}) {
  const [selected, setSelected] = useState(initial);
  return (
    <WorkspaceContext.Provider
      value={{
        workspaces: list,
        currentWorkspaceId: selected,
        currentWorkspace: list.find((w) => w.id === selected) ?? null,
        setCurrentWorkspaceId: (id) => {
          selections.push(id);
          setSelected(id);
        },
        loading: false,
        error: undefined,
        refetch,
      }}
    >
      {children}
    </WorkspaceContext.Provider>
  );
}

/** The root shell's shape: the gate around the routed content. */
async function renderApp({
  path = "/",
  initial = "tea-acme",
  list = [ACME, BETA],
}: { path?: string; initial?: string | null; list?: WorkspaceView[] } = {}) {
  const rootRoute = createRootRoute({
    component: () => (
      <Workspaces initial={initial} list={list}>
        <WorkspaceLoginGate>
          <Outlet />
        </WorkspaceLoginGate>
      </Workspaces>
    ),
  });
  const page = (path: string, text: string) =>
    createRoute({
      getParentRoute: () => rootRoute,
      path,
      component: () => <p>{text}</p>,
    });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      page("/", "Services page"),
      page("/services/srv-1", "Service detail page"),
      page("/settings", "Account settings page"),
      page("/new/workspace", "New workspace page"),
    ]),
    history: createMemoryHistory({ initialEntries: [path] }),
    context: { client: {} as never, session: null },
  });
  await router.load();
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  clearWorkspaceLoginBlocks();
  startGoogleSignIn.mockReset();
  startGoogleSignIn.mockResolvedValue(true);
  leaveMutate.mockReset();
  leaveMutate.mockResolvedValue({ data: { leaveWorkspace: true } });
  refetch.mockReset();
  selections.length = 0;
});

afterEach(() => {
  clearWorkspaceLoginBlocks();
});

describe("WorkspaceLoginGate — two-factor (w22/m402)", () => {
  it("renders the routed content while nothing is blocked", async () => {
    await renderApp();

    expect(await screen.findByText("Services page")).toBeInTheDocument();
    expect(
      screen.queryByTestId("workspace-two-factor-blocked"),
    ).not.toBeInTheDocument();
  });

  it("replaces the routed content with ONE blocked page, naming the workspace and why", async () => {
    await renderApp();
    await screen.findByText("Services page");

    // Many queries fail for the same reason; the page appears once.
    act(() => {
      reportWorkspaceLoginRequired("tea-acme", "twoFactor");
      reportWorkspaceLoginRequired("tea-acme", "twoFactor");
      reportWorkspaceLoginRequired("tea-acme", "twoFactor");
    });

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Acme requires two-factor authentication",
      }),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
    expect(screen.getAllByTestId("workspace-two-factor-blocked")).toHaveLength(
      1,
    );
    expect(
      screen.getByText(
        /you can't access this workspace's resources or settings/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/API keys and SSH keys/)).toBeInTheDocument();
    // Unmounted, so nothing under it keeps querying a workspace that refuses.
    expect(screen.queryByText("Services page")).not.toBeInTheDocument();
    // A link to account security, where Kratos's settings flow enrolls.
    expect(
      screen.getByRole("link", { name: "Set up two-factor authentication" }),
    ).toHaveAttribute("href", "/settings#security");
  });

  it("walls nothing for a refusal from a workspace that is not selected, until it is", async () => {
    await renderApp();
    await screen.findByText("Services page");

    act(() => reportWorkspaceLoginRequired("tea-beta", "twoFactor"));
    expect(screen.getByText("Services page")).toBeInTheDocument();
    expect(
      screen.queryByTestId("workspace-two-factor-blocked"),
    ).not.toBeInTheDocument();
  });

  it("leaves the account pages open so a blocked member can enroll, and re-probes on leaving them", async () => {
    const router = await renderApp();
    await screen.findByText("Services page");
    act(() => reportWorkspaceLoginRequired("tea-acme", "twoFactor"));
    await screen.findByTestId("workspace-two-factor-blocked");

    await act(() => router.navigate({ to: "/settings" }));
    expect(
      await screen.findByText("Account settings page"),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("workspace-two-factor-blocked"),
    ).not.toBeInTheDocument();

    // Creating another workspace is not a workspace query either.
    await act(() =>
      router.navigate({ to: "/new/workspace", search: { attempt: undefined } }),
    );
    expect(await screen.findByText("New workspace page")).toBeInTheDocument();

    // Back in the workspace the recorded block is dropped (the member may have
    // enrolled), so its own queries decide: no report, no page...
    await act(() => router.navigate({ to: "/" }));
    expect(await screen.findByText("Services page")).toBeInTheDocument();
    expect(workspaceLoginBlock("tea-acme")).toBeNull();

    // ...and one more refusal brings the page back. Not a loop: each return
    // needs a fresh answer from the server.
    act(() => reportWorkspaceLoginRequired("tea-acme", "twoFactor"));
    expect(
      await screen.findByTestId("workspace-two-factor-blocked"),
    ).toBeInTheDocument();
  });

  it("'check again' hands the workspace back to its own queries", async () => {
    const user = userEvent.setup();
    await renderApp();
    await screen.findByText("Services page");
    act(() => reportWorkspaceLoginRequired("tea-acme", "twoFactor"));
    await screen.findByTestId("workspace-two-factor-blocked");

    await user.click(
      screen.getByRole("button", { name: "I've turned it on, check again" }),
    );

    expect(await screen.findByText("Services page")).toBeInTheDocument();
    expect(workspaceLoginBlock("tea-acme")).toBeNull();
  });

  describe("leaving the block", () => {
    it("switches to another workspace, off the resource page that belonged to the blocked one", async () => {
      const user = userEvent.setup();
      const router = await renderApp({ path: "/services/srv-1" });
      await screen.findByText("Service detail page");
      act(() => reportWorkspaceLoginRequired("tea-acme", "twoFactor"));
      await screen.findByTestId("workspace-two-factor-blocked");

      await user.click(
        screen.getByRole("combobox", { name: "Switch to another workspace" }),
      );
      const options = await screen.findAllByRole("option");
      expect(options.map((option) => option.textContent)).toEqual(["Beta"]);
      await user.click(options[0]);

      expect(selections).toEqual(["tea-beta"]);
      expect(await screen.findByText("Services page")).toBeInTheDocument();
      expect(router.state.location.pathname).toBe("/");
      expect(
        screen.queryByTestId("workspace-two-factor-blocked"),
      ).not.toBeInTheDocument();
    });

    it("says so when there is no other workspace to switch to", async () => {
      await renderApp({ list: [ACME] });
      await screen.findByText("Services page");
      act(() => reportWorkspaceLoginRequired("tea-acme", "twoFactor"));
      await screen.findByTestId("workspace-two-factor-blocked");

      const picker = screen.getByRole("combobox", {
        name: "Switch to another workspace",
      });
      expect(picker).toBeDisabled();
      expect(picker).toHaveTextContent("No other workspaces");
    });

    it("leaves the workspace after a confirmation, and lands in one the member still has", async () => {
      const user = userEvent.setup();
      const router = await renderApp();
      await screen.findByText("Services page");
      act(() => reportWorkspaceLoginRequired("tea-acme", "twoFactor"));
      await screen.findByTestId("workspace-two-factor-blocked");

      await user.click(screen.getByRole("button", { name: "Leave workspace" }));
      const dialog = await screen.findByRole("alertdialog");
      expect(dialog).toHaveTextContent("Leave this workspace?");
      expect(leaveMutate).not.toHaveBeenCalled();
      await user.click(
        within(dialog).getByRole("button", { name: "Leave workspace" }),
      );

      await waitFor(() =>
        expect(leaveMutate).toHaveBeenCalledWith({
          variables: { workspaceId: "tea-acme" },
        }),
      );
      await waitFor(() => expect(selections).toEqual(["tea-beta"]));
      expect(refetch).toHaveBeenCalled();
      expect(await screen.findByText("Services page")).toBeInTheDocument();
      expect(router.state.location.pathname).toBe("/");
    });

    it("keeps the member here and says why when the server refuses the leave", async () => {
      leaveMutate.mockRejectedValue(
        codedGraphQLError(
          "CONFLICT",
          {},
          "you are the last admin of this workspace",
        ),
      );
      const user = userEvent.setup();
      await renderApp();
      await screen.findByText("Services page");
      act(() => reportWorkspaceLoginRequired("tea-acme", "twoFactor"));
      await screen.findByTestId("workspace-two-factor-blocked");

      await user.click(screen.getByRole("button", { name: "Leave workspace" }));
      await user.click(
        within(await screen.findByRole("alertdialog")).getByRole("button", {
          name: "Leave workspace",
        }),
      );

      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent("Could not leave the workspace");
      expect(alert).toHaveTextContent("last admin");
      expect(selections).toEqual([]);
      expect(
        screen.getByTestId("workspace-two-factor-blocked"),
      ).toBeInTheDocument();
    });
  });

  it("holds the page's skeleton until the workspace list names the workspace", async () => {
    await renderApp({ initial: "tea-acme", list: [] });
    await screen.findByText("Services page");
    act(() => reportWorkspaceLoginRequired("tea-acme", "twoFactor"));

    expect(
      await screen.findByTestId("workspace-two-factor-blocked-skeleton"),
    ).toBeInTheDocument();
    // No name, no action before its data exists.
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});

// AGENTS.md "Shape parity is mandatory": the preview has the page's own frame,
// card, heading row, explanation and action regions, at the same responsive
// classes, so the swap shifts nothing at desktop or narrow-mobile widths.
describe("WorkspaceTwoFactorBlockedSkeleton matches the blocked page (w22/m402)", () => {
  async function renderReadyPage() {
    await renderApp();
    await screen.findByText("Services page");
    act(() => reportWorkspaceLoginRequired("tea-acme", "twoFactor"));
    return screen.findByTestId("workspace-two-factor-blocked");
  }

  it("has the same frame, card, heading row and explanation", async () => {
    const ready = await renderReadyPage();
    const { getByTestId } = render(<WorkspaceTwoFactorBlockedSkeleton />);
    const pending = getByTestId("workspace-two-factor-blocked-skeleton");

    // The outer bounds, padding and max-width.
    expect(pending.className).toBe(ready.className);
    expect(pending.firstElementChild!.className).toBe(
      ready.firstElementChild!.className,
    );
    const pendingCard = pending.querySelector('[data-slot="card"]')!;
    const readyCard = ready.querySelector('[data-slot="card"]')!;
    expect(pendingCard.className).toBe(readyCard.className);
    // The explanation is static copy: the same element and text.
    expect(
      shape(pendingCard.querySelector('[data-slot="card-description"]')!),
    ).toBe(shape(readyCard.querySelector('[data-slot="card-description"]')!));
    // The heading row keeps its icon and the heading's element and classes;
    // only the workspace's name is a placeholder.
    const pendingTitle = pendingCard.querySelector('[data-slot="card-title"]')!;
    const readyTitle = readyCard.querySelector('[data-slot="card-title"]')!;
    expect(pendingTitle.tagName).toBe("H1");
    expect(pendingTitle.className).toBe(readyTitle.className);
    expect(pendingTitle.querySelector("svg")!.getAttribute("class")).toBe(
      readyTitle.querySelector("svg")!.getAttribute("class"),
    );
  });

  it("has the same three action regions in the same order and spacing", async () => {
    const ready = await renderReadyPage();
    const { getByTestId } = render(<WorkspaceTwoFactorBlockedSkeleton />);
    const pending = getByTestId("workspace-two-factor-blocked-skeleton");

    const regions = (root: Element, attribute: string) =>
      [...root.querySelectorAll(`[${attribute}]`)].map((region) => ({
        name: region.getAttribute(attribute),
        className: region.className,
      }));
    expect(regions(pending, "data-skeleton-region")).toEqual(
      regions(ready, "data-region"),
    );
    expect(regions(ready, "data-region").map((r) => r.name)).toEqual([
      "enroll",
      "switch",
      "leave",
    ]);
    // The picker keeps its responsive width (full on narrow, 18rem from sm).
    const pendingPicker = pending.querySelector(
      '[data-skeleton-region="switch"] [data-slot="skeleton"]',
    )!;
    const readyPicker = ready.querySelector('[data-slot="select-trigger"]')!;
    for (const width of ["w-full", "sm:w-72"]) {
      expect(pendingPicker).toHaveClass(width);
      expect(readyPicker).toHaveClass(width);
    }
    // h-9 is the default select trigger's height (data-size="default").
    expect(pendingPicker).toHaveClass("h-9");
    expect(readyPicker).toHaveAttribute("data-size", "default");
    // The enroll and re-check buttons are one h-9 row that wraps like the page's.
    const pendingRow = pending.querySelector(
      '[data-skeleton-region="enroll"] > div',
    )!;
    const readyRow = ready.querySelector('[data-region="enroll"] > div')!;
    expect(pendingRow.className).toBe(readyRow.className);
    expect(pendingRow.children).toHaveLength(readyRow.children.length);
    for (const box of pendingRow.children) expect(box).toHaveClass("h-9");
    // The static hint is the same text in the same classes.
    expect(
      shape(pending.querySelector('[data-skeleton-region="enroll"] > p')!),
    ).toBe(shape(ready.querySelector('[data-region="enroll"] > p')!));
  });
});

// w7/m465: the Google login requirement's blocked page, the 2FA page's
// sibling behind the same gate and record.
describe("WorkspaceLoginGate — Google login (w7/m465)", () => {
  it("replaces the routed content with ONE Google login page, naming the workspace and why", async () => {
    await renderApp();
    await screen.findByText("Services page");

    act(() => {
      reportWorkspaceLoginRequired("tea-acme", "googleLogin");
      reportWorkspaceLoginRequired("tea-acme", "googleLogin");
    });

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Acme requires Google login",
      }),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
    expect(
      screen.getAllByTestId("workspace-google-login-blocked"),
    ).toHaveLength(1);
    expect(
      screen.queryByTestId("workspace-two-factor-blocked"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(/the Google account of your bex email address/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /you can't access this workspace's resources or settings/,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Services page")).not.toBeInTheDocument();
    // Signing in is the way back, not account security's 2FA enrollment.
    expect(
      screen.queryByRole("link", { name: "Set up two-factor authentication" }),
    ).not.toBeInTheDocument();
  });

  it("signs the session in with Google and comes back to this page", async () => {
    const user = userEvent.setup();
    await renderApp({ path: "/services/srv-1" });
    await screen.findByText("Service detail page");
    act(() => reportWorkspaceLoginRequired("tea-acme", "googleLogin"));
    await screen.findByTestId("workspace-google-login-blocked");

    const signIn = screen.getByRole("button", { name: "Sign in with Google" });
    await user.click(signIn);

    expect(startGoogleSignIn).toHaveBeenCalledTimes(1);
    expect(startGoogleSignIn).toHaveBeenCalledWith(window.location.href);
    // The page is leaving for Google: no second click while it goes.
    expect(signIn).toBeDisabled();
    expect(
      screen.getByText("You'll come back to this page once you've signed in."),
    ).toBeInTheDocument();
  });

  it("says so when the sign-in does not start, and lets the member retry", async () => {
    startGoogleSignIn.mockRejectedValueOnce(new Error("not configured"));
    const user = userEvent.setup();
    await renderApp();
    await screen.findByText("Services page");
    act(() => reportWorkspaceLoginRequired("tea-acme", "googleLogin"));
    await screen.findByTestId("workspace-google-login-blocked");

    await user.click(
      screen.getByRole("button", { name: "Sign in with Google" }),
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Couldn't start signing in with Google. Try again.",
    );
    const retry = screen.getByRole("button", { name: "Sign in with Google" });
    expect(retry).toBeEnabled();
    await user.click(retry);
    expect(startGoogleSignIn).toHaveBeenCalledTimes(2);
  });

  it("offers the same switch and leave exits", async () => {
    const user = userEvent.setup();
    const router = await renderApp({ path: "/services/srv-1" });
    await screen.findByText("Service detail page");
    act(() => reportWorkspaceLoginRequired("tea-acme", "googleLogin"));
    await screen.findByTestId("workspace-google-login-blocked");

    expect(
      screen.getByRole("button", { name: "Leave workspace" }),
    ).toBeInTheDocument();
    await user.click(
      screen.getByRole("combobox", { name: "Switch to another workspace" }),
    );
    await user.click((await screen.findAllByRole("option"))[0]);

    expect(selections).toEqual(["tea-beta"]);
    expect(await screen.findByText("Services page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
  });

  it("leaves the account pages open, and drops the block on leaving them", async () => {
    const router = await renderApp();
    await screen.findByText("Services page");
    act(() => reportWorkspaceLoginRequired("tea-acme", "googleLogin"));
    await screen.findByTestId("workspace-google-login-blocked");

    await act(() => router.navigate({ to: "/settings" }));
    expect(
      await screen.findByText("Account settings page"),
    ).toBeInTheDocument();

    await act(() => router.navigate({ to: "/" }));
    expect(await screen.findByText("Services page")).toBeInTheDocument();
    expect(workspaceLoginBlock("tea-acme")).toBeNull();
  });

  // A member who fails both requirements is refused for Google first; once
  // signed in with Google, the server names the second factor instead.
  it("shows the page of the requirement the server named last", async () => {
    await renderApp();
    await screen.findByText("Services page");

    act(() => reportWorkspaceLoginRequired("tea-acme", "googleLogin"));
    expect(
      await screen.findByTestId("workspace-google-login-blocked"),
    ).toBeInTheDocument();

    act(() => reportWorkspaceLoginRequired("tea-acme", "twoFactor"));
    expect(
      await screen.findByTestId("workspace-two-factor-blocked"),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("workspace-google-login-blocked"),
    ).not.toBeInTheDocument();
  });

  it("holds the page's skeleton until the workspace list names the workspace", async () => {
    await renderApp({ initial: "tea-acme", list: [] });
    await screen.findByText("Services page");
    act(() => reportWorkspaceLoginRequired("tea-acme", "googleLogin"));

    expect(
      await screen.findByTestId("workspace-google-login-blocked-skeleton"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});

// AGENTS.md "Shape parity is mandatory", for the Google login page: the same
// frame, card, heading row, explanation and regions as the ready page, at the
// same responsive classes (the picker is full width on narrow screens and
// 18rem from sm; every other compared class holds at every width).
describe("WorkspaceGoogleLoginBlockedSkeleton matches the blocked page (w7/m465)", () => {
  async function renderReadyPage() {
    await renderApp();
    await screen.findByText("Services page");
    act(() => reportWorkspaceLoginRequired("tea-acme", "googleLogin"));
    return screen.findByTestId("workspace-google-login-blocked");
  }

  it("has the same frame, card, heading row and explanation", async () => {
    const ready = await renderReadyPage();
    const { getByTestId } = render(<WorkspaceGoogleLoginBlockedSkeleton />);
    const pending = getByTestId("workspace-google-login-blocked-skeleton");

    expect(pending.className).toBe(ready.className);
    expect(pending.firstElementChild!.className).toBe(
      ready.firstElementChild!.className,
    );
    const pendingCard = pending.querySelector('[data-slot="card"]')!;
    const readyCard = ready.querySelector('[data-slot="card"]')!;
    expect(pendingCard.className).toBe(readyCard.className);
    expect(
      shape(pendingCard.querySelector('[data-slot="card-description"]')!),
    ).toBe(shape(readyCard.querySelector('[data-slot="card-description"]')!));
    const pendingTitle = pendingCard.querySelector('[data-slot="card-title"]')!;
    const readyTitle = readyCard.querySelector('[data-slot="card-title"]')!;
    expect(pendingTitle.tagName).toBe("H1");
    expect(pendingTitle.className).toBe(readyTitle.className);
    expect(pendingTitle.querySelector("svg")!.getAttribute("class")).toBe(
      readyTitle.querySelector("svg")!.getAttribute("class"),
    );
  });

  it("has the same three regions in the same order and spacing", async () => {
    const ready = await renderReadyPage();
    const { getByTestId } = render(<WorkspaceGoogleLoginBlockedSkeleton />);
    const pending = getByTestId("workspace-google-login-blocked-skeleton");

    const regions = (root: Element, attribute: string) =>
      [...root.querySelectorAll(`[${attribute}]`)].map((region) => ({
        name: region.getAttribute(attribute),
        className: region.className,
      }));
    expect(regions(pending, "data-skeleton-region")).toEqual(
      regions(ready, "data-region"),
    );
    expect(regions(ready, "data-region").map((r) => r.name)).toEqual([
      "sign-in",
      "switch",
      "leave",
    ]);
    // The sign-in button is one h-9 row that wraps like the page's.
    const pendingRow = pending.querySelector(
      '[data-skeleton-region="sign-in"] > div',
    )!;
    const readyRow = ready.querySelector('[data-region="sign-in"] > div')!;
    expect(pendingRow.className).toBe(readyRow.className);
    expect(pendingRow.children).toHaveLength(readyRow.children.length);
    for (const box of pendingRow.children) expect(box).toHaveClass("h-9");
    expect(readyRow.querySelector("button")).toHaveClass("h-9");
    // The static hint is the same text in the same classes.
    expect(
      shape(pending.querySelector('[data-skeleton-region="sign-in"] > p')!),
    ).toBe(shape(ready.querySelector('[data-region="sign-in"] > p')!));
    // The shared exits keep their responsive picker width.
    const pendingPicker = pending.querySelector(
      '[data-skeleton-region="switch"] [data-slot="skeleton"]',
    )!;
    const readyPicker = ready.querySelector('[data-slot="select-trigger"]')!;
    for (const width of ["w-full", "sm:w-72"]) {
      expect(pendingPicker).toHaveClass(width);
      expect(readyPicker).toHaveClass(width);
    }
  });
});

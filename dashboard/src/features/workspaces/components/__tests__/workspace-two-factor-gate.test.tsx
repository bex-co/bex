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
import { WorkspaceTwoFactorGate } from "@/features/workspaces/components/workspace-two-factor-gate";
import { WorkspaceTwoFactorBlockedSkeleton } from "@/features/workspaces/components/workspace-two-factor-blocked-skeleton";
import { shape } from "@/test/card-shape";
import {
  isWorkspaceTwoFactorBlocked,
  reportWorkspaceTwoFactorRequired,
  clearWorkspaceTwoFactorBlocks,
} from "@/features/workspaces/lib/two-factor-block";
import type { WorkspaceView } from "@/features/workspaces/types";

const leaveMutate = vi.hoisted(() => vi.fn());
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [leaveMutate],
}));

function workspace(id: string, name: string): WorkspaceView {
  return {
    id,
    name,
    slug: name.toLowerCase(),
    plan: "pro",
    role: "developer",
    createdAt: null,
    requireTwoFactor: true,
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
        <WorkspaceTwoFactorGate>
          <Outlet />
        </WorkspaceTwoFactorGate>
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
  clearWorkspaceTwoFactorBlocks();
  leaveMutate.mockReset();
  leaveMutate.mockResolvedValue({ data: { leaveWorkspace: true } });
  refetch.mockReset();
  selections.length = 0;
});

afterEach(() => {
  clearWorkspaceTwoFactorBlocks();
});

describe("WorkspaceTwoFactorGate (w22/m402)", () => {
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
      reportWorkspaceTwoFactorRequired("tea-acme");
      reportWorkspaceTwoFactorRequired("tea-acme");
      reportWorkspaceTwoFactorRequired("tea-acme");
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

    act(() => reportWorkspaceTwoFactorRequired("tea-beta"));
    expect(screen.getByText("Services page")).toBeInTheDocument();
    expect(
      screen.queryByTestId("workspace-two-factor-blocked"),
    ).not.toBeInTheDocument();
  });

  it("leaves the account pages open so a blocked member can enroll, and re-probes on leaving them", async () => {
    const router = await renderApp();
    await screen.findByText("Services page");
    act(() => reportWorkspaceTwoFactorRequired("tea-acme"));
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
    expect(isWorkspaceTwoFactorBlocked("tea-acme")).toBe(false);

    // ...and one more refusal brings the page back. Not a loop: each return
    // needs a fresh answer from the server.
    act(() => reportWorkspaceTwoFactorRequired("tea-acme"));
    expect(
      await screen.findByTestId("workspace-two-factor-blocked"),
    ).toBeInTheDocument();
  });

  it("'check again' hands the workspace back to its own queries", async () => {
    const user = userEvent.setup();
    await renderApp();
    await screen.findByText("Services page");
    act(() => reportWorkspaceTwoFactorRequired("tea-acme"));
    await screen.findByTestId("workspace-two-factor-blocked");

    await user.click(
      screen.getByRole("button", { name: "I've turned it on, check again" }),
    );

    expect(await screen.findByText("Services page")).toBeInTheDocument();
    expect(isWorkspaceTwoFactorBlocked("tea-acme")).toBe(false);
  });

  describe("leaving the block", () => {
    it("switches to another workspace, off the resource page that belonged to the blocked one", async () => {
      const user = userEvent.setup();
      const router = await renderApp({ path: "/services/srv-1" });
      await screen.findByText("Service detail page");
      act(() => reportWorkspaceTwoFactorRequired("tea-acme"));
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
      act(() => reportWorkspaceTwoFactorRequired("tea-acme"));
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
      act(() => reportWorkspaceTwoFactorRequired("tea-acme"));
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
      act(() => reportWorkspaceTwoFactorRequired("tea-acme"));
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
    act(() => reportWorkspaceTwoFactorRequired("tea-acme"));

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
    act(() => reportWorkspaceTwoFactorRequired("tea-acme"));
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

import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AutomationsPage } from "@/features/agent-automations/pages/automations-page";
import { AutomationsListSkeleton } from "@/features/agent-automations/components/automations-list-skeleton";
import { automation } from "@/features/agent-automations/components/__tests__/fixtures";
import {
  automationActions,
  NO_ACTIONS,
} from "@/features/agent-automations/lib/display";
import { ROUTE_SKELETON_MANIFEST } from "@/common/lib/route-skeleton-manifest";

const list = vi.hoisted(() => ({ useAgentAutomations: vi.fn() }));
const ops = vi.hoisted(() => ({
  toggle: vi.fn(async () => true),
  runNow: vi.fn(),
  takeOver: vi.fn(),
  remove: vi.fn(),
  busy: false,
}));
const viewer = vi.hoisted(() => ({
  current: null as unknown as {
    canCreate: boolean;
    actionsFor: (createdBy: string) => unknown;
  },
}));

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({
      children,
      to,
      ...props
    }: {
      children: ReactNode;
      to: string;
      className?: string;
      "aria-label"?: string;
    }) => (
      <a href={to} {...props}>
        {children}
      </a>
    ),
  };
});
vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("@/features/agent-automations/hooks/use-agent-automations", () => ({
  useAgentAutomations: (...args: unknown[]) =>
    list.useAgentAutomations(...args),
}));
vi.mock("@/features/agent-automations/hooks/use-automation-operations", () => ({
  useAutomationOperations: () => ops,
}));
vi.mock("@/features/agent-automations/hooks/use-automation-viewer", () => ({
  useAutomationViewer: () => viewer.current,
}));

function listResult(over: Record<string, unknown> = {}) {
  return {
    automations: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
    loadMore: vi.fn(),
    loadingMore: false,
    hasMore: false,
    ...over,
  };
}

beforeEach(() => {
  ops.toggle.mockClear();
  list.useAgentAutomations.mockReset();
  list.useAgentAutomations.mockReturnValue(listResult());
  viewer.current = {
    canCreate: true,
    actionsFor: (createdBy: string) =>
      automationActions({
        canCreate: true,
        canManage: false,
        isCreator: createdBy === "usr-1",
      }),
  };
});

describe("AutomationsPage", () => {
  it("asks for the caller's own automations on Mine and the workspace's on All", async () => {
    const user = userEvent.setup();
    render(<AutomationsPage />);
    expect(list.useAgentAutomations).toHaveBeenLastCalledWith(true);
    await user.click(screen.getByRole("tab", { name: "All" }));
    expect(list.useAgentAutomations).toHaveBeenLastCalledWith(false);
  });

  it("explains an empty list and offers Create", () => {
    render(<AutomationsPage />);
    expect(screen.getByTestId("automations-empty")).toHaveTextContent(
      "You have no automations yet",
    );
    expect(
      within(screen.getByTestId("automations-empty")).getByRole("link", {
        name: "Create automation",
      }),
    ).toHaveAttribute("href", "/automations/new");
  });

  it("shows trigger in words, next run, last run, paused reason and a keyboard-operable switch", async () => {
    const user = userEvent.setup();
    list.useAgentAutomations.mockReturnValue(
      listResult({
        automations: [
          automation({
            lastFireAt: "2026-11-06T08:00:00Z",
            lastSessionId: "ags-9",
          }),
          automation({
            id: "aut-2",
            name: "weekly-report",
            enabled: true,
            pausedReason: "repeated_failures",
            nextFireAt: null,
            createdBy: "usr-2",
          }),
        ],
      }),
    );
    render(<AutomationsPage />);
    const rows = within(screen.getByTestId("automation-list")).getAllByRole(
      "listitem",
    );
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("nightly-deps");
    expect(rows[0]).toHaveTextContent("Every day at 09:00 · Europe/Berlin");
    expect(rows[0]).toHaveTextContent("Next run:");
    expect(
      within(rows[0]).getByRole("link", { name: "View session" }),
    ).toBeInTheDocument();

    expect(rows[1]).toHaveTextContent("Paused: repeated failures");

    // The creator's switch pauses; keyboard operable.
    const own = within(rows[0]).getByRole("switch", {
      name: "Pause nightly-deps",
    });
    expect(own).toBeChecked();
    own.focus();
    await user.keyboard(" ");
    expect(ops.toggle).toHaveBeenCalledWith(
      expect.objectContaining({ id: "aut-1" }),
    );

    // Someone else's automation is read-only for a plain developer.
    expect(
      within(rows[1]).getByRole("switch", { name: "Resume weekly-report" }),
    ).toBeDisabled();
  });

  it("disables Create until the viewer's role allows it", () => {
    viewer.current = { canCreate: false, actionsFor: () => NO_ACTIONS };
    render(<AutomationsPage />);
    expect(
      screen.getByRole("button", { name: "Create automation" }),
    ).toBeDisabled();
  });

  it("retries a failed list read instead of showing it as empty", async () => {
    const { uncodedGraphQLError } = await import("@/test/mocks/apollo");
    const refetch = vi.fn();
    list.useAgentAutomations.mockReturnValue(
      listResult({ error: uncodedGraphQLError("boom"), refetch }),
    );
    render(<AutomationsPage />);
    expect(screen.queryByTestId("automations-empty")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("pages with Load more only while the server has another page", async () => {
    const loadMore = vi.fn();
    list.useAgentAutomations.mockReturnValue(
      listResult({ automations: [automation()], hasMore: true, loadMore }),
    );
    const { rerender } = render(<AutomationsPage />);
    await userEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(loadMore).toHaveBeenCalledTimes(1);
    list.useAgentAutomations.mockReturnValue(
      listResult({ automations: [automation()], hasMore: false }),
    );
    rerender(<AutomationsPage />);
    expect(
      screen.queryByRole("button", { name: "Load more" }),
    ).not.toBeInTheDocument();
  });

  it("shows the forbidden message for a member who cannot start sessions", async () => {
    const { codedGraphQLError } = await import("@/test/mocks/apollo");
    list.useAgentAutomations.mockReturnValue(
      listResult({ error: codedGraphQLError("FORBIDDEN") }),
    );
    render(<AutomationsPage />);
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Only members with the developer role or higher",
    );
  });
});

describe("AutomationsListSkeleton matches the ready page (dashboard/AGENTS.md)", () => {
  it("carries the same heading, description, tabs, create button and row shell", () => {
    list.useAgentAutomations.mockReturnValue(
      listResult({ automations: [automation()] }),
    );
    const ready = render(<AutomationsPage />);
    const readyRow = within(ready.container).getAllByRole("listitem")[0];
    const readyText = (el: HTMLElement) =>
      [
        ...el.querySelectorAll(
          "h1, header p, [role=tab], header button, header a",
        ),
      ].map((n) => n.textContent);
    const readyTexts = readyText(ready.container);
    const readyOuter = ready.container.firstElementChild?.className;
    const readyRowClass = readyRow.className;
    const readyHeaderClass = ready.container.querySelector("header")?.className;
    ready.unmount();

    const pending = render(<AutomationsListSkeleton />);
    expect(readyText(pending.container)).toEqual(readyTexts);
    expect(pending.container.firstElementChild?.className).toBe(readyOuter);
    expect(pending.container.querySelector("header")?.className).toBe(
      readyHeaderClass,
    );
    const pendingRow = pending.container.querySelector("li");
    expect(pendingRow?.className).toBe(readyRowClass);
    // Nothing is actionable while pending.
    expect(
      screen.getByRole("button", { name: "Create automation" }),
    ).toBeDisabled();
    expect(
      screen.getAllByRole("tab").every((t) => t.hasAttribute("disabled")),
    ).toBe(true);
  });

  it("reserves every region the manifest promises", () => {
    const entry = ROUTE_SKELETON_MANIFEST["/automations"];
    const { container } = render(<AutomationsListSkeleton />);
    if (entry.kind !== "render") throw new Error("expected a render route");
    for (const region of entry.regions) {
      expect(
        container.querySelector(`[data-skeleton-region="${region}"]`),
      ).not.toBeNull();
    }
    expect(
      container.querySelector('[data-route-skeleton="automations-list"]'),
    ).not.toBeNull();
  });
});

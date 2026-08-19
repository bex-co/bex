import { beforeEach, describe, it, expect, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionList } from "@/features/agent-sessions/components/session-list";
import type {
  AgentSessionPhase,
  AgentSessionView,
} from "@/features/agent-sessions/types";
import { agentSessionView } from "@/test/mocks/agent-session";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    title,
    search,
  }: {
    children: React.ReactNode;
    title?: string;
    search?: unknown;
  }) => (
    <a href="#session" title={title} data-search={JSON.stringify(search)}>
      {children}
    </a>
  ),
}));

const { toastSuccess } = vi.hoisted(() => ({ toastSuccess: vi.fn() }));
vi.mock("sonner", () => ({
  toast: { success: toastSuccess, error: vi.fn() },
}));

// The per-row archive toggle drives Apollo mutations; the list's own rendering
// is what's under test, so the hook is inert.
const archiveMock = vi.fn().mockResolvedValue(undefined);
const unarchiveMock = vi.fn().mockResolvedValue(undefined);
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({
    archive: archiveMock,
    unarchive: unarchiveMock,
  }),
}));

/** `task` is a convenience override for the nested `agentConfig.task`. */
function view(
  over: Partial<AgentSessionView> & { task?: string } = {},
): AgentSessionView {
  return agentSessionView({ status: "working", ...over });
}

describe("SessionList", () => {
  beforeEach(() => {
    archiveMock.mockClear();
    unarchiveMock.mockClear();
    toastSuccess.mockClear();
  });

  it("renders the empty state (not the table) when there are no sessions", () => {
    render(<SessionList sessions={[]} loading={false} />);
    expect(screen.getByText("No agent sessions yet")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows a loading skeleton before the first sessions arrive", () => {
    const { container } = render(<SessionList sessions={[]} loading />);
    expect(container.querySelector('[data-slot="skeleton"]')).toBeTruthy();
    expect(screen.queryByText("No agent sessions yet")).not.toBeInTheDocument();
  });

  it("shows the error state when the load fails with no rows", () => {
    const onRetry = vi.fn();
    render(
      <SessionList
        sessions={[]}
        loading={false}
        error={new Error("boom")}
        onRetry={onRetry}
      />,
    );
    expect(
      screen.getByText("Couldn't load agent sessions"),
    ).toBeInTheDocument();
    screen.getByRole("button", { name: "Retry" }).click();
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("explains filtered empty states and offers to clear the filters", async () => {
    const user = userEvent.setup();
    const onClearFilters = vi.fn();
    const { rerender } = render(
      <SessionList
        sessions={[]}
        loading={false}
        archiveFilter="true"
        onClearFilters={onClearFilters}
      />,
    );
    expect(screen.getByText("No archived sessions")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(onClearFilters).toHaveBeenCalledTimes(1);

    rerender(
      <SessionList
        sessions={[]}
        loading={false}
        phase="failed"
        onClearFilters={onClearFilters}
      />,
    );
    expect(screen.getByText("No matching sessions")).toBeInTheDocument();
  });

  it("renders a per-phase chip with the localized phase label", () => {
    const cases: Array<[AgentSessionPhase, string]> = [
      ["creating", "Creating"],
      ["running", "Running"],
      ["completed", "Completed"],
      ["failed", "Failed"],
      ["canceled", "Canceled"],
    ];
    const sessions = cases.map(([phase], i) =>
      view({ id: `as-${i}`, phase, task: `task ${i}` }),
    );
    render(<SessionList sessions={sessions} loading={false} />);
    for (const [, label] of cases) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it("links a session with a PR number to its prUrl, and shows an em dash otherwise", () => {
    render(
      <SessionList
        loading={false}
        sessions={[
          view({
            id: "as-pr",
            task: "with pr",
            prNumber: 42,
            prUrl: "https://github.com/acme/widgets/pull/42",
          }),
          view({ id: "as-nopr", task: "no pr", prNumber: null }),
        ]}
      />,
    );

    const link = screen.getByRole("link", { name: /#42/ });
    expect(link).toHaveAttribute(
      "href",
      "https://github.com/acme/widgets/pull/42",
    );
    expect(link).toHaveAttribute("target", "_blank");

    // The PR-less row's cell renders an em dash placeholder, not a badge.
    const noPrRow = screen.getByText("no pr").closest("tr")!;
    expect(within(noPrRow).getByText("—")).toBeInTheDocument();
  });

  it("renders a PR number without a link when prUrl is absent", () => {
    render(
      <SessionList
        loading={false}
        sessions={[view({ prNumber: 7, prUrl: null })]}
      />,
    );
    expect(screen.getByText("#7")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /#7/ })).not.toBeInTheDocument();
  });

  it("falls back to the session id when the task prompt is empty", () => {
    render(
      <SessionList
        loading={false}
        sessions={[view({ id: "as-empty", task: "" })]}
      />,
    );
    expect(screen.getByText("as-empty")).toBeInTheDocument();
  });

  it("carries membership and phase filters into a session detail link", () => {
    render(
      <SessionList
        loading={false}
        archiveFilter="true"
        phase="failed"
        sessions={[view({ id: "as-filtered", task: "inspect failure" })]}
      />,
    );
    expect(screen.getByText("inspect failure").closest("a")).toHaveAttribute(
      "data-search",
      JSON.stringify({ fromArchived: "true", fromPhase: "failed" }),
    );
  });

  // ADR065 D1/D6: an archived row is marked, and each row carries the
  // archive/unarchive toggle that drives the mutation + a list refresh.
  it("marks archived rows and offers per-row archive/unarchive", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    render(
      <SessionList
        loading={false}
        onChanged={onChanged}
        sessions={[
          view({ id: "as-live", task: "still working" }),
          view({
            id: "as-arch",
            task: "put away",
            phase: "completed",
            archivedAt: new Date().toISOString(),
            isArchived: true,
          }),
        ]}
      />,
    );

    // The archived row is badged; the working-set row is not.
    const archivedRow = screen.getByText("put away").closest("tr")!;
    expect(within(archivedRow).getByText("Archived")).toBeInTheDocument();
    const liveRow = screen.getByText("still working").closest("tr")!;
    expect(within(liveRow).queryByText("Archived")).not.toBeInTheDocument();

    // Row actions: archive on the live row, unarchive on the archived one.
    await user.click(within(liveRow).getByRole("button", { name: "Archive" }));
    expect(archiveMock).toHaveBeenCalledWith("as-live");
    const archiveToast = toastSuccess.mock.calls.find(
      ([message]) => message === "Session archived",
    );
    expect(archiveToast?.[1]?.action.label).toBe("Undo");
    archiveToast?.[1]?.action.onClick();
    await waitFor(() => expect(unarchiveMock).toHaveBeenCalledWith("as-live"));
    await user.click(
      within(archivedRow).getByRole("button", { name: "Unarchive" }),
    );
    expect(unarchiveMock).toHaveBeenCalledWith("as-arch");
    expect(onChanged).toHaveBeenCalledTimes(2);
  });

  it("keeps the row action busy until its refresh completes", async () => {
    const user = userEvent.setup();
    let finishRefresh: (() => void) | undefined;
    const onChanged = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          finishRefresh = resolve;
        }),
    );
    render(
      <SessionList
        loading={false}
        onChanged={onChanged}
        sessions={[view({ id: "as-live", task: "wait for refresh" })]}
      />,
    );
    const action = screen.getByRole("button", { name: "Archive" });
    await user.click(action);
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(action).toBeDisabled();
    finishRefresh?.();
    await waitFor(() => expect(action).toBeEnabled());
  });
});

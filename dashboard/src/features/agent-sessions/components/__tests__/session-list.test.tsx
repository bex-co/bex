import { beforeEach, describe, it, expect, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionList } from "@/features/agent-sessions/components/session-list";
import type {
  AgentSessionPhase,
  AgentSessionView,
} from "@/features/agent-sessions/types";
import {
  agentSessionView,
  repoLessAgentSessionView,
} from "@/test/mocks/agent-session";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    title,
    search,
    className,
  }: {
    children: React.ReactNode;
    title?: string;
    search?: unknown;
    className?: string;
  }) => (
    <a
      href="#session"
      title={title}
      className={className}
      data-search={JSON.stringify(search)}
    >
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

/** The chat-only shape: bex-api clears repo AND branch when there is no repo. */
function repoLessView(
  over: Partial<AgentSessionView> & { task?: string } = {},
): AgentSessionView {
  return repoLessAgentSessionView({ status: "working", ...over });
}

/** The first row's meta line, read as text. */
function metaText(): string {
  return screen.getAllByTestId("agent-session-meta")[0]?.textContent ?? "";
}

describe("SessionList", () => {
  beforeEach(() => {
    archiveMock.mockClear();
    unarchiveMock.mockClear();
    toastSuccess.mockClear();
  });

  it("renders a quiet empty line (not the dashed Bot card or table)", () => {
    render(<SessionList sessions={[]} loading={false} />);
    expect(
      screen.getByText("Sessions you start will show up here."),
    ).toBeInTheDocument();
    expect(screen.queryByText("No agent sessions yet")).not.toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows recents-shaped skeletons before the first sessions arrive", () => {
    render(<SessionList sessions={[]} loading />);
    expect(
      screen.getByTestId("agent-sessions-recents-skeleton"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Sessions you start will show up here."),
    ).not.toBeInTheDocument();
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
    expect(screen.getByText("Couldn't load sessions.")).toBeInTheDocument();
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
        archiveFilter="archived"
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

  it("uses human status phrases on recents, not the ten-phase chip set", () => {
    render(
      <SessionList
        loading={false}
        sessions={[
          view({ id: "as-redisp", phase: "redispatching", task: "keep going" }),
          view({
            id: "as-pr",
            phase: "completed",
            prNumber: 3,
            task: "ship it",
          }),
        ]}
      />,
    );
    expect(screen.getByText(/Working…/)).toBeInTheDocument();
    expect(screen.getByText(/PR is ready/)).toBeInTheDocument();
    expect(screen.queryByText("Redispatching")).not.toBeInTheDocument();
    expect(screen.queryByText("Phase")).not.toBeInTheDocument();
    expect(screen.getByText("keep going").closest("a")).toHaveAttribute(
      "href",
      "#session",
    );
  });

  it("keeps phase chips on the denser Archived/All table", () => {
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
    render(
      <SessionList sessions={sessions} loading={false} archiveFilter="all" />,
    );
    expect(screen.getByText("Phase")).toBeInTheDocument();
    for (const [, label] of cases) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it("links a session with a PR number to its prUrl, and shows an em dash otherwise", () => {
    render(
      <SessionList
        loading={false}
        archiveFilter="all"
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
        archiveFilter="all"
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
        archiveFilter="archived"
        phase="failed"
        sessions={[view({ id: "as-filtered", task: "inspect failure" })]}
      />,
    );
    expect(screen.getByText("inspect failure").closest("a")).toHaveAttribute(
      "data-search",
      JSON.stringify({ fromArchived: "archived", fromPhase: "failed" }),
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
        archiveFilter="all"
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
        archiveFilter="all"
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

  // These read the rendered TEXT of the line, so an empty value between two
  // separators fails (see MetaLine in session-list.tsx).
  describe("meta line separators (w1/m90)", () => {
    it("keeps a repo-backed recents row exactly as it was", () => {
      render(<SessionList loading={false} sessions={[view({ task: "a" })]} />);
      expect(metaText()).toMatch(/^Working… · acme\/widgets · .+$/);
    });

    it("drops the separator with the value on a repo-less recents row", () => {
      render(
        <SessionList
          loading={false}
          sessions={[repoLessView({ task: "a" })]}
        />,
      );
      expect(metaText()).toMatch(/^Working… · .+$/);
      expect(metaText()).not.toMatch(/·\s*·/);
      expect(metaText()).not.toMatch(/·\s*$/);
    });

    it("renders repo · branch · agent on a repo-backed table row", () => {
      render(
        <SessionList
          loading={false}
          archiveFilter="all"
          sessions={[view({ task: "a" })]}
        />,
      );
      expect(metaText()).toBe("acme/widgets · bex-agent/fix · claude");
    });

    it("renders exactly one separator for a repo with no branch", () => {
      render(
        <SessionList
          loading={false}
          archiveFilter="all"
          sessions={[view({ task: "a", branch: "" })]}
        />,
      );
      expect(metaText()).toBe("acme/widgets · claude");
    });

    it("renders no separator at all for a repo-less table row", () => {
      render(
        <SessionList
          loading={false}
          archiveFilter="all"
          sessions={[repoLessView({ task: "a" })]}
        />,
      );
      expect(metaText()).toBe("claude");
      expect(metaText()).not.toContain("·");
    });

    it("still names a repo-less row by its prompt", () => {
      render(
        <SessionList
          loading={false}
          sessions={[repoLessView({ task: "explain the mapper" })]}
        />,
      );
      expect(screen.getByText("explain the mapper")).toBeInTheDocument();
    });
  });
});

import type { ReactNode } from "react";
import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AgentSessionsPageContent } from "../agents";

const useAgentSessions = vi.fn();

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => vi.fn(),
    // The PR-state filter's options are links; expose the search they carry.
    Link: ({
      children,
      search,
      ...rest
    }: {
      children: ReactNode;
      search?: unknown;
      "aria-current"?: "true";
    }) => (
      <a
        href="/agents"
        aria-current={rest["aria-current"]}
        data-search={JSON.stringify(search)}
      >
        {children}
      </a>
    ),
  };
});

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => children,
}));

vi.mock("@/features/agent-sessions/components/new-session-composer", () => ({
  NewSessionComposer: () => <div data-testid="new-session-composer" />,
}));

vi.mock("@/features/agent-sessions/components/session-list", () => ({
  SessionList: () => <div data-testid="session-list" />,
}));

vi.mock("@/features/agent-sessions/hooks/use-agent-sessions", () => ({
  useAgentSessions: (...args: unknown[]) => useAgentSessions(...args),
}));

beforeEach(() => {
  useAgentSessions.mockReset();
  useAgentSessions.mockReturnValue({
    sessions: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
    loadMore: vi.fn(),
    loadingMore: false,
    hasMore: false,
  });
});

describe("AgentSessionsPageContent", () => {
  it("keeps the default page composer-only so Recents exists only in the rail", () => {
    render(<AgentSessionsPageContent />);

    expect(screen.getByTestId("new-session-composer")).toBeInTheDocument();
    expect(screen.queryByTestId("session-list")).not.toBeInTheDocument();
    expect(screen.queryByText("Recent")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("group", { name: "Filter by pull request state" }),
    ).not.toBeInTheDocument();
    expect(useAgentSessions).not.toHaveBeenCalled();
  });

  // w9/m386: `?prState=` opens the history list, narrowed to that PR state.
  it("lists only the sessions whose PR is in the requested state", () => {
    render(<AgentSessionsPageContent prState="merged" />);

    expect(
      screen.queryByTestId("new-session-composer"),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("session-list")).toBeInTheDocument();
    expect(useAgentSessions).toHaveBeenCalledWith(
      expect.objectContaining({ prStates: ["merged"] }),
    );
    const filter = screen.getByRole("group", {
      name: "Filter by pull request state",
    });
    expect(
      within(filter).getByRole("link", { name: "Merged" }),
    ).toHaveAttribute("aria-current", "true");
  });

  it("offers the PR-state filter on every history list, keeping its filters", () => {
    render(<AgentSessionsPageContent archived="all" tag="ci" />);

    expect(useAgentSessions).toHaveBeenCalledWith(
      expect.objectContaining({ prStates: undefined }),
    );
    const filter = screen.getByRole("group", {
      name: "Filter by pull request state",
    });
    expect(within(filter).getByRole("link", { name: "Any" })).toHaveAttribute(
      "aria-current",
      "true",
    );
    expect(
      within(filter)
        .getByRole("link", { name: "Closed" })
        .getAttribute("data-search"),
    ).toBe(JSON.stringify({ archived: "all", tag: "ci", prState: "closed" }));
  });

  // w4/m249: exactly one h1 in either mode, with the translated page title.
  it("has exactly one h1 on the default page and in list mode", () => {
    const { unmount } = render(<AgentSessionsPageContent />);
    const composerH1 = screen.getAllByRole("heading", { level: 1 });
    expect(composerH1).toHaveLength(1);
    expect(composerH1[0]).toHaveTextContent("Agent sessions");
    unmount();

    render(<AgentSessionsPageContent archived="all" />);
    const listH1 = screen.getAllByRole("heading", { level: 1 });
    expect(listH1).toHaveLength(1);
    expect(listH1[0]).toHaveTextContent("Agent sessions");
  });

  it.each([
    [{ archived: "archived" as const, phase: undefined }, "Archived"],
    [{ archived: "all" as const, phase: undefined }, "All"],
    [{ archived: undefined, phase: "failed" as const }, "Recent"],
  ])("keeps filtered history reachable for %o", (search, heading) => {
    render(<AgentSessionsPageContent {...search} />);

    expect(
      screen.queryByTestId("new-session-composer"),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: heading })).toBeInTheDocument();
    expect(screen.getByTestId("session-list")).toBeInTheDocument();
    expect(useAgentSessions).toHaveBeenCalledWith({
      poll: false,
      archived: search.archived,
      phases: search.phase ? [search.phase] : undefined,
    });
  });
});

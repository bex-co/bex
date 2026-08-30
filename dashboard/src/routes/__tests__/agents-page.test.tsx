import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AgentSessionsPageContent } from "../agents";

const useAgentSessions = vi.fn();

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@tanstack/react-router")>();
  return { ...actual, useNavigate: () => vi.fn() };
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
    expect(useAgentSessions).not.toHaveBeenCalled();
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

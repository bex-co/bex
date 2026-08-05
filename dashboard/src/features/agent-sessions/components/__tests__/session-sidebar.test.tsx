import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionSidebar } from "@/features/agent-sessions/components/session-sidebar";
import { agentSessionStatusPhrase } from "@/features/agent-sessions/lib/mapper";
import type {
  AgentSessionPhase,
  AgentSessionView,
} from "@/features/agent-sessions/types";

const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    to,
    search,
    ...rest
  }: {
    children: React.ReactNode;
    to?: string;
    search?: Record<string, unknown>;
  } & Record<string, unknown>) => (
    <a
      href={`${to ?? "#"}${search?.view ? `?view=${String(search.view)}` : ""}`}
      {...(rest as object)}
    >
      {children}
    </a>
  ),
  useNavigate: () => navigate,
}));

const sessionsState: { sessions: AgentSessionView[]; loading: boolean } = {
  sessions: [],
  loading: false,
};
vi.mock("@/features/agent-sessions/hooks/use-agent-sessions", () => ({
  useAgentSessions: () => sessionsState,
}));

function view(over: Partial<AgentSessionView> = {}): AgentSessionView {
  const phase = (over.phase ?? "completed") as AgentSessionPhase;
  return {
    id: "as-1",
    ownerId: "tea-1",
    repo: "acme/widgets",
    branch: "bex-agent/fix",
    agentConfig: {
      agent: "claude",
      model: null,
      modelEndpoint: null,
      task: "refactor the mapper",
      template: null,
    },
    sandboxId: null,
    phase,
    status: phase,
    headSha: null,
    prUrl: null,
    prNumber: null,
    evidence: null,
    turns: 1,
    deliveryMode: null,
    failureReason: null,
    createdAt: "2026-08-05T00:00:00Z",
    updatedAt: "2026-08-05T00:01:00Z",
    canceledAt: null,
    isTerminal:
      phase === "completed" || phase === "failed" || phase === "canceled",
    isSteerable: phase === "completed" || phase === "failed",
    ...over,
  };
}

beforeEach(() => {
  navigate.mockClear();
  sessionsState.loading = false;
  sessionsState.sessions = [];
});

describe("agentSessionStatusPhrase", () => {
  it("maps phase + PR presence onto the Devin-style phrase", () => {
    expect(agentSessionStatusPhrase({ phase: "completed", prNumber: 6 })).toBe(
      "prReady",
    );
    expect(
      agentSessionStatusPhrase({ phase: "completed", prNumber: null }),
    ).toBe("completed");
    expect(agentSessionStatusPhrase({ phase: "failed", prNumber: null })).toBe(
      "failed",
    );
    expect(
      agentSessionStatusPhrase({ phase: "canceled", prNumber: null }),
    ).toBe("canceled");
    expect(
      agentSessionStatusPhrase({ phase: "canceling", prNumber: null }),
    ).toBe("canceled");
    for (const phase of [
      "running",
      "creating",
      "resuming",
      "redispatching",
    ] as const) {
      expect(agentSessionStatusPhrase({ phase, prNumber: null })).toBe(
        "working",
      );
    }
  });
});

describe("SessionSidebar", () => {
  it("shows human status phrases and links the PR number straight to GitHub", () => {
    sessionsState.sessions = [
      view({
        id: "as-pr",
        prNumber: 6,
        prUrl: "https://github.com/acme/widgets/pull/6",
        agentConfig: { ...view().agentConfig, task: "ship the PR one" },
      }),
      view({
        id: "as-run",
        phase: "running",
        agentConfig: { ...view().agentConfig, task: "still working one" },
      }),
      view({
        id: "as-fail",
        phase: "failed",
        agentConfig: { ...view().agentConfig, task: "broken one" },
      }),
    ];
    render(<SessionSidebar activeId="as-run" />);

    expect(screen.getByText("PR is ready")).toBeInTheDocument();
    expect(screen.getByText("Working…")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();

    // The PR number is a DIRECT external GitHub link, not an internal route.
    const pr = screen.getByRole("link", { name: /#6/ });
    expect(pr).toHaveAttribute(
      "href",
      "https://github.com/acme/widgets/pull/6",
    );
  });

  it("filters the Recent list by title/repo through the search toggle", async () => {
    sessionsState.sessions = [
      view({
        id: "as-a",
        agentConfig: { ...view().agentConfig, task: "wire up metrics" },
      }),
      view({
        id: "as-b",
        agentConfig: { ...view().agentConfig, task: "tighten hero copy" },
      }),
    ];
    render(<SessionSidebar activeId="" />);
    expect(screen.getByText("wire up metrics")).toBeInTheDocument();
    expect(screen.getByText("tighten hero copy")).toBeInTheDocument();

    await userEvent.click(
      screen.getByRole("button", { name: /search sessions/i }),
    );
    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "metrics" } });

    expect(screen.getByText("wire up metrics")).toBeInTheDocument();
    expect(screen.queryByText("tighten hero copy")).not.toBeInTheDocument();
  });

  it("exposes a view-all action reaching the standalone list", () => {
    sessionsState.sessions = [view()];
    render(<SessionSidebar activeId="" />);
    const viewAll = screen.getByRole("link", { name: /view all sessions/i });
    expect(viewAll.getAttribute("href")).toContain("view=list");
  });
});

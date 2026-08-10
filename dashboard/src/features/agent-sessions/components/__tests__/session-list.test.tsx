import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { SessionList } from "@/features/agent-sessions/components/session-list";
import type {
  AgentSessionPhase,
  AgentSessionView,
} from "@/features/agent-sessions/types";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    title,
  }: {
    children: React.ReactNode;
    title?: string;
  }) => (
    <a href="#session" title={title}>
      {children}
    </a>
  ),
}));

/** `task` is a convenience override for the nested `agentConfig.task`. */
function view({
  task = "refactor the mapper",
  ...over
}: Partial<AgentSessionView> & { task?: string } = {}): AgentSessionView {
  return {
    id: "as-1",
    ownerId: "tea-1",
    repo: "acme/widgets",
    branch: "bex-agent/fix",
    agentConfig: {
      agent: "claude",
      model: null,
      modelEndpoint: null,
      task,
      template: null,
    },
    sandboxId: null,
    sshAddress: null,
    phase: "running",
    status: "working",
    headSha: null,
    prUrl: null,
    prNumber: null,
    evidence: null,
    turns: 0,
    deliveryMode: null,
    failureReason: null,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    canceledAt: null,
    isTerminal: false,
    isSteerable: false,
    ...over,
  };
}

describe("SessionList", () => {
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
    render(
      <SessionList sessions={[]} loading={false} error={new Error("boom")} />,
    );
    expect(
      screen.getByText("Couldn't load agent sessions"),
    ).toBeInTheDocument();
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
});

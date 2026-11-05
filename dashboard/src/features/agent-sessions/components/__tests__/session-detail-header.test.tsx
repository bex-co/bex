import { beforeEach, describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionDetailHeader } from "@/features/agent-sessions/components/session-detail-header";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import {
  agentSessionView,
  repoLessAgentSessionView,
} from "@/test/mocks/agent-session";
import { AgentModelSource } from "@/graphql/definitions";
import type { AgentSessionModelCost } from "@/features/agent-sessions/hooks/use-agent-session-model-cost";

const navigateMock = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    search,
    ...rest
  }: {
    children: React.ReactNode;
    search?: unknown;
  }) => (
    <a {...rest} data-search={JSON.stringify(search)}>
      {children}
    </a>
  ),
  useNavigate: () => navigateMock,
}));
const archiveMock = vi.fn().mockResolvedValue(undefined);
const unarchiveMock = vi.fn().mockResolvedValue(undefined);
const deleteMock = vi.fn().mockResolvedValue(undefined);
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({
    cancel: vi.fn(),
    archive: archiveMock,
    unarchive: unarchiveMock,
    deleteSession: deleteMock,
  }),
}));

// The header's model cost (ADR091 D6) reads its own query.
const modelCost = vi.hoisted(() => ({
  cost: null as AgentSessionModelCost | null,
  loading: false,
}));
vi.mock("@/features/agent-sessions/hooks/use-agent-session-model-cost", () => ({
  useAgentSessionModelCost: () => ({
    cost: modelCost.cost,
    loading: modelCost.loading,
  }),
}));

function view(over: Partial<AgentSessionView> = {}): AgentSessionView {
  return agentSessionView({ phase: "completed", turns: 1, ...over });
}

/** The chat-only shape: bex-api clears repo AND branch when there is no repo. */
function repoLessView(
  over: Partial<AgentSessionView> & { task?: string } = {},
): AgentSessionView {
  return repoLessAgentSessionView({ phase: "completed", turns: 1, ...over });
}

function heading(): HTMLElement {
  return screen.getByRole("heading", { level: 1 });
}

describe("SessionDetailHeader", () => {
  beforeEach(() => {
    modelCost.cost = null;
    modelCost.loading = false;
    navigateMock.mockClear();
    archiveMock.mockClear();
    unarchiveMock.mockClear();
    deleteMock.mockClear();
  });

  it("preserves the originating list filters in the Back link", () => {
    render(
      <SessionDetailHeader
        session={view()}
        backSearch={{ archived: "archived", phase: "failed" }}
      />,
    );
    expect(screen.getByLabelText("All sessions")).toHaveAttribute(
      "data-search",
      JSON.stringify({ archived: "archived", phase: "failed" }),
    );
  });

  it("shows singular turn copy for a single-turn session", () => {
    render(<SessionDetailHeader session={view({ turns: 1 })} />);
    expect(screen.getByText("1 turn")).toBeInTheDocument();
    expect(screen.queryByText("1 turns")).not.toBeInTheDocument();
  });

  it("shows plural turns for multi-turn sessions", () => {
    render(<SessionDetailHeader session={view({ turns: 3 })} />);
    expect(screen.getByText("3 turns")).toBeInTheDocument();
  });

  // With the inline PR card gone, this badge is the session's only PR
  // affordance, so it has to survive.
  it("links the draft PR through the #N badge when the session opened one", () => {
    render(
      <SessionDetailHeader
        session={view({
          prNumber: 7,
          prUrl: "https://github.com/acme/widgets/pull/7",
        })}
      />,
    );
    const link = screen.getByRole("link", { name: /#7/ });
    expect(link).toHaveAttribute(
      "href",
      "https://github.com/acme/widgets/pull/7",
    );
  });

  // The default (opted-out) session has no PR at all — the badge must stay away
  // rather than render an empty or dead reference.
  it("renders no PR badge for a session that never asked for one", () => {
    render(<SessionDetailHeader session={view()} />);
    expect(screen.queryByText(/#\d/)).not.toBeInTheDocument();
  });

  // w10/m15: a session closed after its idle TTL carries no storage size and
  // no Pin control, just the ordinary completed header.
  it("shows no Pin control or snapshot size for a closed session", () => {
    render(<SessionDetailHeader session={view({ sandboxId: null })} />);
    expect(
      screen.queryByRole("button", { name: /pin/i }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/KiB|MiB/)).not.toBeInTheDocument();
  });

  // ADR065 D1/D6: the overflow menu carries Archive (any phase) and, on a
  // finished session, the destructive Delete behind a confirmation dialog.
  it("returns to the sessions list after archive and refreshes an unarchive in place", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    const { rerender } = render(
      <SessionDetailHeader session={view()} onChanged={onChanged} />,
    );
    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(screen.getByRole("menuitem", { name: /^archive$/i }));
    expect(archiveMock).toHaveBeenCalledTimes(1);
    await waitFor(() =>
      expect(navigateMock).toHaveBeenCalledWith({ to: "/agents" }),
    );
    expect(onChanged).not.toHaveBeenCalled();

    rerender(
      <SessionDetailHeader
        session={view({
          archivedAt: new Date().toISOString(),
          isArchived: true,
        })}
        onChanged={onChanged}
      />,
    );
    expect(screen.getByText("Archived")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(screen.getByRole("menuitem", { name: /unarchive/i }));
    expect(unarchiveMock).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(navigateMock).toHaveBeenCalledTimes(1);
  });

  it("offers Delete only on a finished session, behind a confirmation", async () => {
    const user = userEvent.setup();
    // A live session must cancel first — no Delete in the menu.
    const { rerender } = render(
      <SessionDetailHeader session={view({ phase: "running" })} />,
    );
    await user.click(screen.getByRole("button", { name: "More actions" }));
    expect(
      screen.queryByRole("menuitem", { name: /delete/i }),
    ).not.toBeInTheDocument();
    await user.keyboard("{Escape}");

    rerender(<SessionDetailHeader session={view()} />); // completed
    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(screen.getByRole("menuitem", { name: /delete/i }));
    // The confirm dialog gates the destructive call.
    expect(deleteMock).not.toHaveBeenCalled();
    expect(screen.getByText("Delete this session?")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Delete session" }));
    expect(deleteMock).toHaveBeenCalledTimes(1);
    await waitFor(() =>
      expect(navigateMock).toHaveBeenCalledWith({
        to: "/agents",
        replace: true,
      }),
    );
  });

  // w1/m90: a repo-less session rendered `<h1 class="…"></h1>` — the element was
  // there, so a "the heading mounted" assertion would have passed against the
  // broken build. Every assertion below reads TEXT.
  describe("session identity (w1/m90)", () => {
    it("names a repo-backed session by its repository, branch row intact", () => {
      const { container } = render(<SessionDetailHeader session={view()} />);
      expect(heading()).toHaveTextContent("acme/widgets");
      expect(heading()).toHaveAttribute("title", "acme/widgets");
      expect(screen.getByText("bex-agent/fix")).toBeInTheDocument();
      expect(container.querySelector(".lucide-git-branch")).not.toBeNull();
    });

    it("names a repo-less session by its prompt", () => {
      render(
        <SessionDetailHeader
          session={repoLessView({ task: "explain the mapper" })}
        />,
      );
      expect(heading().textContent).toBe("explain the mapper");
    });

    it("renders no GitBranch icon at all for a repo-less session", () => {
      const { container } = render(
        <SessionDetailHeader session={repoLessView()} />,
      );
      expect(container.querySelector(".lucide-git-branch")).toBeNull();
      // The rest of the meta row is unaffected.
      expect(screen.getByText("1 turn")).toBeInTheDocument();
    });

    it("falls back to a localized name when a session has neither", () => {
      render(<SessionDetailHeader session={repoLessView({ task: "" })} />);
      expect(heading().textContent).toBe("Untitled session");
    });

    it("truncates a long prompt but keeps the full text in the title", () => {
      const task = `${"refactor the mapper ".repeat(20)}end`;
      render(<SessionDetailHeader session={repoLessView({ task })} />);
      expect(heading().textContent!.endsWith("…")).toBe(true);
      expect(heading().textContent!.length).toBeLessThan(task.length);
      expect(heading()).toHaveAttribute("title", task);
    });
  });
});

describe("SessionDetailHeader model cost (ADR091 D6)", () => {
  const turn = (n: number, costUsd: string) => ({
    __typename: "AgentSessionTurnModelCost" as const,
    turn: n,
    costUsd,
    requests: 2,
    inputTokens: 1200,
    outputTokens: 300,
    cacheReadTokens: 40,
    cacheWriteTokens: 2,
    models: ["deepseek/deepseek-v4"],
  });

  beforeEach(() => {
    modelCost.cost = null;
    modelCost.loading = false;
  });

  it("shows the session total and each turn's cost", async () => {
    modelCost.cost = {
      __typename: "AgentSessionModelCost",
      sessionId: "ags-1",
      modelSource: AgentModelSource.Platform,
      costUsd: "0.42",
      requests: 4,
      // Turn 2's billed requests round to zero cents: "<$0.01", never free.
      turns: [turn(1, "0.42"), turn(2, "0.00")],
    };
    const user = userEvent.setup();
    render(<SessionDetailHeader session={view({ turns: 2 })} />);
    await user.click(
      screen.getByRole("button", { name: /\$0\.42 model cost/ }),
    );
    const list = await screen.findByRole("list", {
      name: "Model cost by turn",
    });
    expect(list).toHaveTextContent("Turn 1$0.42");
    expect(list).toHaveTextContent("Turn 2<$0.01");
    expect(list).toHaveTextContent(
      "1,200 input · 300 output · 42 cache tokens",
    );
    expect(
      screen.getByText("Billed at the model provider's cost + 30%."),
    ).toBeInTheDocument();
  });

  it("reads Billed by your provider for a session on the workspace's own key", () => {
    modelCost.cost = {
      __typename: "AgentSessionModelCost",
      sessionId: "ags-1",
      modelSource: AgentModelSource.Byo,
      costUsd: "0.00",
      requests: 0,
      turns: [],
    };
    render(<SessionDetailHeader session={view()} />);
    expect(screen.getByText("Billed by your provider")).toBeInTheDocument();
    expect(screen.queryByText(/model cost/)).toBeNull();
  });

  it("reads $0.00 for a platform session with no requests yet", () => {
    modelCost.cost = {
      __typename: "AgentSessionModelCost",
      sessionId: "ags-1",
      modelSource: AgentModelSource.Platform,
      costUsd: "0.00",
      requests: 0,
      turns: [],
    };
    render(<SessionDetailHeader session={view()} />);
    expect(screen.getByText("$0.00 model cost")).toBeInTheDocument();
  });

  it("holds the cost slot with a skeleton while it loads", () => {
    modelCost.loading = true;
    render(<SessionDetailHeader session={view()} />);
    expect(
      screen.getByTestId("session-model-cost-skeleton"),
    ).toBeInTheDocument();
  });

  // The ready header keeps the skeleton's slot when the cost can't be read,
  // so the meta row never shifts (dashboard AGENTS.md skeleton rule).
  it("keeps the slot's width when the cost is unavailable", () => {
    modelCost.loading = true;
    const { unmount } = render(<SessionDetailHeader session={view()} />);
    const skeletonClass = screen
      .getByTestId("session-model-cost-skeleton")
      .className.split(" ")
      .filter((c) => /^[hw]-/.test(c));
    unmount();
    modelCost.loading = false;
    render(<SessionDetailHeader session={view()} />);
    const placeholder = screen.getByTestId("session-model-cost-placeholder");
    expect(placeholder).toHaveAttribute("aria-hidden", "true");
    expect(placeholder).toHaveClass(...skeletonClass);
  });
});

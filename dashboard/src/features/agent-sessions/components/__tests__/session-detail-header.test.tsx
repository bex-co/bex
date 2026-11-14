import { beforeEach, describe, it, expect, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionDetailHeader } from "@/features/agent-sessions/components/session-detail-header";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import { hydrateAcrossBoundary } from "@/test/hydration";
import {
  agentSessionView,
  repoLessAgentSessionView,
} from "@/test/mocks/agent-session";
import { AgentModelSource } from "@/graphql/definitions";
import type { AgentSessionModelCost } from "@/features/agent-sessions/hooks/use-agent-session-model-cost";
import type { AppliedSecret } from "@/features/agent-sessions/hooks/use-agent-session-applied-secrets";

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
const setTagsMock = vi.fn().mockResolvedValue(undefined);
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({
    cancel: vi.fn(),
    archive: archiveMock,
    unarchive: unarchiveMock,
    deleteSession: deleteMock,
    setTags: setTagsMock,
  }),
}));
// Who started it (w7/m296) resolves through the workspace roster.
vi.mock("@/common/hooks/use-workspace-subject-label", () => ({
  useWorkspaceSubjectLabel: (subject: string) =>
    subject === "kratos-alice" ? "alice@example.com" : subject,
}));

// "Started by automation" reads the automation's name on its own (ADR101 D9).
const automationName = vi.hoisted(() => ({ name: null as string | null }));
vi.mock("@/features/agent-automations/hooks/use-agent-automation", () => ({
  useAgentAutomation: () => ({
    automation: automationName.name ? { name: automationName.name } : null,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
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

// What the latest turn received (w9/m385) reads the detail query on its own.
const appliedSecrets = vi.hoisted(() => ({ current: [] as AppliedSecret[] }));
vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-applied-secrets",
  () => ({
    useAgentSessionAppliedSecrets: () => appliedSecrets.current,
  }),
);

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
    appliedSecrets.current = [];
    modelCost.cost = null;
    modelCost.loading = false;
    navigateMock.mockClear();
    archiveMock.mockClear();
    unarchiveMock.mockClear();
    deleteMock.mockClear();
    setTagsMock.mockReset();
    setTagsMock.mockResolvedValue(undefined);
  });

  // w9/m385: the latest turn's secrets, each with why it received it.
  it("lists the secrets the latest turn received with their sources", async () => {
    appliedSecrets.current = [
      { name: "STRIPE_KEY", source: "explicit" },
      { name: "API_DEPLOY_KEY", source: "repo" },
      { name: "ORG_TOKEN", source: "workspace" },
      { name: "TEST_TOKEN", source: "inline" },
    ];
    render(
      <SessionDetailHeader
        session={view({ secretNames: ["STRIPE_KEY", "TEST_TOKEN"] })}
      />,
    );
    expect(screen.queryByTestId("session-secret-names")).toBeNull();
    const trigger = screen.getByTestId("session-applied-secrets");
    expect(trigger).toHaveTextContent("4 secrets");
    await userEvent.click(trigger);
    const list = await screen.findByRole("list", {
      name: "Secrets in the latest turn",
    });
    expect(
      within(list)
        .getAllByRole("listitem")
        .map((item) => item.textContent),
    ).toEqual([
      "STRIPE_KEY· selected",
      "API_DEPLOY_KEY· repository",
      "ORG_TOKEN· workspace",
      "TEST_TOKEN· this session",
    ]);
  });

  it("counts the attached secrets until a turn has applied any", () => {
    render(
      <SessionDetailHeader session={view({ secretNames: ["STRIPE_KEY"] })} />,
    );
    expect(screen.getByTestId("session-secret-names")).toHaveTextContent(
      "1 secret",
    );
    expect(screen.queryByTestId("session-applied-secrets")).toBeNull();
  });

  it("shows no secrets when the session has none", () => {
    render(<SessionDetailHeader session={view()} />);
    expect(screen.queryByTestId("session-secret-names")).toBeNull();
    expect(screen.queryByTestId("session-applied-secrets")).toBeNull();
  });

  // w7/m296: every session says who started it, as Devin's sessions do.
  it("names the member who started the session", () => {
    render(
      <SessionDetailHeader session={view({ createdBy: "kratos-alice" })} />,
    );
    expect(screen.getByTestId("started-by")).toHaveTextContent(
      "Started by alice@example.com",
    );
  });

  it("falls back to the raw subject when no member matches (an API key)", () => {
    render(<SessionDetailHeader session={view({ createdBy: "client-ci" })} />);
    expect(screen.getByTestId("started-by")).toHaveTextContent(
      "Started by client-ci",
    );
  });

  it("names the member an automation's session runs as", () => {
    automationName.name = "nightly-deps";
    render(
      <SessionDetailHeader
        session={view({ automationId: "aut-1", createdBy: "kratos-alice" })}
      />,
    );
    expect(screen.getByTestId("started-by")).toHaveTextContent(
      "Started by automation nightly-deps for alice@example.com",
    );
    automationName.name = null;
  });

  it("shows the session's tags and replaces them from Edit tags", async () => {
    render(
      <SessionDetailHeader
        session={view({ id: "as-9", tags: ["bug", "ui"] })}
      />,
    );
    expect(screen.getByTestId("session-tags")).toHaveTextContent("bug, ui");

    await userEvent.click(screen.getByRole("button", { name: "More actions" }));
    await userEvent.click(
      await screen.findByRole("menuitem", { name: "Edit tags" }),
    );
    const input = await screen.findByRole("textbox", { name: "Tags" });
    expect(input).toHaveValue("bug, ui");
    await userEvent.clear(input);
    await userEvent.type(input, "triage, p1 backend");
    await userEvent.click(screen.getByRole("button", { name: "Save tags" }));

    await waitFor(() =>
      expect(setTagsMock).toHaveBeenCalledWith("as-9", [
        "triage",
        "p1",
        "backend",
      ]),
    );
    expect(
      screen.queryByRole("textbox", { name: "Tags" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the editor open with the server's reason when a tag is refused", async () => {
    setTagsMock.mockRejectedValue(
      new Error("each tag must match [a-z0-9:_-]{1,64}"),
    );
    render(<SessionDetailHeader session={view({ tags: [] })} />);
    await userEvent.click(screen.getByRole("button", { name: "More actions" }));
    await userEvent.click(
      await screen.findByRole("menuitem", { name: "Edit tags" }),
    );
    await userEvent.type(
      await screen.findByRole("textbox", { name: "Tags" }),
      "Bad Tag",
    );
    await userEvent.click(screen.getByRole("button", { name: "Save tags" }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Tags" })).toBeInTheDocument();
  });

  it("offers no Edit tags on an archived session", async () => {
    render(
      <SessionDetailHeader
        session={view({ archivedAt: "2026-08-05T00:00:00Z", isArchived: true })}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "More actions" }));
    expect(
      await screen.findByRole("menuitem", { name: "Unarchive" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Edit tags" }),
    ).not.toBeInTheDocument();
  });

  it("links a session an automation started back to that automation", () => {
    automationName.name = "nightly-deps";
    render(<SessionDetailHeader session={view({ automationId: "aut-1" })} />);
    const started = screen.getByTestId("started-by");
    expect(started).toHaveTextContent("Started by automation nightly-deps");
    expect(within(started).getByText("nightly-deps")).toHaveAttribute(
      "to",
      "/automations/$automationId",
    );
    automationName.name = null;
  });

  it("shows no automation note for a session a person started", () => {
    render(<SessionDetailHeader session={view()} />);
    expect(screen.queryByTestId("started-by")).not.toBeInTheDocument();
  });

  // ADR111 D4: the latest turn booted from a repository environment build.
  it("marks a session that started from an environment snapshot", () => {
    render(
      <SessionDetailHeader
        session={view({ environmentBuildId: "arb-0123456789abcdefghij" })}
      />,
    );
    const chip = screen.getByTestId("session-environment-build");
    expect(chip).toHaveTextContent("Started from snapshot");
    expect(chip).toHaveAttribute(
      "title",
      "The latest turn started from environment build arb-0123456789abcdefghij, with the setup steps already run.",
    );
    // It sits in the one-line meta row with the other chips.
    expect(chip.closest(".h-4")).not.toBeNull();
  });

  it("shows no snapshot chip for a turn that ran the setup steps", () => {
    render(<SessionDetailHeader session={view()} />);
    expect(screen.queryByTestId("session-environment-build")).toBeNull();
    expect(screen.queryByText("Started from snapshot")).toBeNull();
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

  // The route skeleton draws the metadata as one 16px line; the loaded row
  // scrolls sideways on a narrow screen instead of wrapping (w9 browser check).
  it("keeps the metadata on one line that scrolls instead of wrapping", () => {
    render(<SessionDetailHeader session={view({ turns: 1 })} />);
    const row = screen.getByText("1 turn").parentElement!;
    expect(row).toHaveClass("h-4", "whitespace-nowrap", "overflow-x-auto");
    expect(row).not.toHaveClass("flex-wrap");
  });

  // w9/080: the running-session duration ticks off Date.now(), so the SSR
  // pass and hydration read different clocks. The text must hydrate without
  // React error #418 (the client clock takes over after mount).
  it("hydrates a running session's ticking duration without a mismatch", () => {
    const createdAt = "2026-11-06T10:00:00Z";
    const session = view({
      phase: "running",
      isTerminal: false,
      createdAt,
      updatedAt: createdAt,
      canceledAt: null,
    });
    const { html, recovered } = hydrateAcrossBoundary(
      <SessionDetailHeader session={session} />,
      {
        serverNow: Date.parse(createdAt) + 61_000,
        clientNow: Date.parse(createdAt) + 64_000,
      },
    );
    expect(html).toContain("1m 1s");
    expect(recovered).toEqual([]);
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

  // w9/m386: the PR's state sits beside the #N link once bex has read it.
  it.each([
    ["open", "Open"],
    ["merged", "Merged"],
    ["closed", "Closed"],
  ] as const)("shows a %s PR's state beside its link", (prState, label) => {
    render(
      <SessionDetailHeader
        session={view({
          prNumber: 7,
          prUrl: "https://github.com/acme/widgets/pull/7",
          prState,
        })}
      />,
    );
    const link = screen.getByRole("link", { name: /#7/ });
    const badge = screen.getByText(label);
    expect(badge).toHaveAttribute("data-pr-state", prState);
    expect(link).not.toContainElement(badge);
    expect(link.parentElement).toContainElement(badge);
  });

  it("shows no PR state until bex has read it", () => {
    render(
      <SessionDetailHeader
        session={view({
          prNumber: 7,
          prUrl: "https://github.com/acme/widgets/pull/7",
          prState: null,
        })}
      />,
    );
    expect(screen.getByRole("link", { name: /#7/ })).toBeInTheDocument();
    expect(document.querySelector("[data-pr-state]")).toBeNull();
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

  // w9/m178: once the session reports outcome.cost.totalUSD the header shows
  // that total (model + sandbox compute), like the session page's spend line,
  // and the popover splits it; an older server keeps the model-only label.
  const withTotal = (cost: {
    modelUSD: string;
    computeUSD: string | null;
    totalUSD: string | null;
  }) => {
    const base = view({ turns: 2 });
    return { ...base, outcome: { ...base.outcome!, cost } } as AgentSessionView;
  };

  it("shows the total cost with its model and compute split", async () => {
    modelCost.cost = {
      __typename: "AgentSessionModelCost",
      sessionId: "ags-1",
      modelSource: AgentModelSource.Platform,
      costUsd: "0.42",
      requests: 4,
      turns: [turn(1, "0.42")],
    };
    const user = userEvent.setup();
    render(
      <SessionDetailHeader
        session={withTotal({
          modelUSD: "0.42",
          computeUSD: "0.0712",
          totalUSD: "0.4912",
        })}
      />,
    );
    const trigger = screen.getByRole("button", {
      name: /\$0\.4912 total cost/,
    });
    expect(screen.queryByText(/model cost/)).toBeNull();
    await user.click(trigger);
    expect(
      await screen.findByText("Model $0.42 · compute $0.0712"),
    ).toBeInTheDocument();
  });

  it("shows the total for a session on the workspace's own key once compute is billed", () => {
    modelCost.cost = {
      __typename: "AgentSessionModelCost",
      sessionId: "ags-1",
      modelSource: AgentModelSource.Byo,
      costUsd: "0.00",
      requests: 0,
      turns: [],
    };
    render(
      <SessionDetailHeader
        session={withTotal({
          modelUSD: "0.00",
          computeUSD: "0.0031",
          totalUSD: "0.0031",
        })}
      />,
    );
    expect(screen.getByTestId("session-cost-label")).toHaveTextContent(
      "$0.0031 total cost",
    );
    expect(screen.queryByText("Billed by your provider")).toBeNull();
  });

  it("keeps the model-only label for a server without totalUSD", () => {
    modelCost.cost = {
      __typename: "AgentSessionModelCost",
      sessionId: "ags-1",
      modelSource: AgentModelSource.Platform,
      costUsd: "0.42",
      requests: 4,
      turns: [turn(1, "0.42")],
    };
    render(
      <SessionDetailHeader
        session={withTotal({
          modelUSD: "0.42",
          computeUSD: null,
          totalUSD: null,
        })}
      />,
    );
    expect(
      screen.getByRole("button", { name: /\$0\.42 model cost/ }),
    ).toBeInTheDocument();
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

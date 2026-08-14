import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NewSessionComposer } from "@/features/agent-sessions/components/new-session-composer";
import {
  AgentSessionError,
  AgentSessionsUnavailableError,
} from "@/features/agent-sessions/lib/errors";
import type { AgentSessionView } from "@/features/agent-sessions/types";

const mockNavigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

const create = vi.fn();
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({ create }),
}));

// The @ picker's repo source — the git feature's installation repo list.
vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => ({
    repos: [
      {
        id: 1,
        fullName: "acme/widgets",
        private: false,
        defaultBranch: "main",
        htmlUrl: "https://github.com/acme/widgets",
        cloneUrl: "",
      },
      {
        id: 2,
        fullName: "acme/anvils",
        private: true,
        defaultBranch: "develop",
        htmlUrl: "https://github.com/acme/anvils",
        cloneUrl: "",
      },
    ],
    loading: false,
    error: undefined,
  }),
}));

let priorSessions: AgentSessionView[] = [];
vi.mock("@/features/agent-sessions/hooks/use-agent-sessions", () => ({
  useAgentSessions: () => ({
    sessions: priorSessions,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));

beforeEach(() => {
  priorSessions = [];
  mockNavigate.mockReset();
  create.mockReset();
  create.mockResolvedValue({
    session: { id: "as-new" },
    ticket: "tkt",
    url: null,
    expiresAt: null,
  });
});

/** Type a valid task into the prompt box. */
async function typeTask(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("Task"), "  refactor the mapper  ");
}

/** Insert a repo mention through the @ toolbar → Repositories → item. */
async function pickRepo(
  user: ReturnType<typeof userEvent.setup>,
  match: RegExp = /widgets/,
) {
  await user.click(
    screen.getByRole("button", { name: "Mention a repository or session" }),
  );
  await user.click(await screen.findByRole("option", { name: /Repositories/ }));
  await user.click(await screen.findByRole("option", { name: match }));
}

describe("NewSessionComposer", () => {
  it("submits createAgentSession with the mentioned repo, derived branch, and trimmed task, then navigates", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({
        ownerId: "tea-1",
        repo: "acme/widgets",
        branch: "bex-agent/refactor-the-mapper",
        agent: "claude",
        task: "refactor the mapper",
        egressAllowlist: [],
      }),
    );
    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/agents/$agentSessionId",
        params: { agentSessionId: "as-new" },
      }),
    );
  });

  it("submits on Enter and inserts a newline on Shift+Enter", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    // Shift+Enter adds a newline without submitting (chat-composer style).
    await user.keyboard("{Shift>}{Enter}{/Shift}");
    expect(create).not.toHaveBeenCalled();

    // A bare Enter submits the prompt.
    await user.keyboard("{Enter}");

    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ repo: "acme/widgets" }),
    );
  });

  it("keeps Send disabled until the task text is non-empty", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);

    const send = screen.getByRole("button", { name: "Start session" });
    expect(send).toBeDisabled();

    await user.type(screen.getByLabelText("Task"), "do a thing");
    expect(send).toBeEnabled();
  });

  it("nudges at the @ button instead of submitting when no repo is mentioned", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    expect(
      await screen.findByText("Pick a repository with @ first."),
    ).toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();
  });

  it("opens the toolbar mention after text without requiring manual whitespace", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await user.type(screen.getByLabelText("Task"), "fix this");

    await user.click(
      screen.getByRole("button", { name: "Mention a repository or session" }),
    );

    expect(
      await screen.findByRole("option", { name: /Repositories/ }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Task")).toHaveTextContent("fix this @");
  });

  it("opens categories on a typed @, filters, and inserts an atomic inline mention", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    const task = screen.getByLabelText("Task");

    // A typed `@` at a word boundary opens the category level.
    await user.type(task, "fix the bug @");
    expect(
      await screen.findByRole("option", { name: /Repositories/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: /Sessions/ }),
    ).toBeInTheDocument();

    // Enter selects the highlighted category and swaps `@` for the token.
    await user.keyboard("{Enter}");
    expect(task).toHaveTextContent("fix the bug @repos:");

    // Typing after the token fuzzy-filters the repo list.
    await user.keyboard("anvils");
    expect(
      await screen.findByRole("option", { name: /anvils/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: /widgets/ }),
    ).not.toBeInTheDocument();

    // The highlighted repo shows the readiness preview footer.
    expect(screen.getByText("acme/anvils")).toBeInTheDocument();
    expect(screen.getByText("Connected via GitHub App")).toBeInTheDocument();
    expect(screen.getByText("Default branch: develop")).toBeInTheDocument();

    // Enter replaces the typed token with a mention node at the same caret.
    await user.keyboard("{Enter}");
    expect(task).toHaveTextContent("fix the bug @acme/anvils");
    expect(
      task.querySelector('[data-type="mention"][data-id="repo:acme/anvils"]'),
    ).toBeInTheDocument();

    // The inline mention's repo is what the create receives.
    await user.click(screen.getByRole("button", { name: "Start session" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0].repo).toBe("acme/anvils");
  });

  it("replaces the prior inline repo because a session has one checkout", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    const task = screen.getByLabelText("Task");
    await pickRepo(user, /anvils/);

    expect(
      task.querySelector('[data-id="repo:acme/widgets"]'),
    ).not.toBeInTheDocument();
    expect(
      task.querySelector('[data-id="repo:acme/anvils"]'),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Start session" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ repo: "acme/anvils" }),
    );
  });

  it("keeps session context inline and serializes its id into the prompt", async () => {
    priorSessions = [agentSession("as-prior", "Investigate flaky tests")];
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    await user.click(
      screen.getByRole("button", { name: "Mention a repository or session" }),
    );
    await user.click(await screen.findByRole("option", { name: /Sessions/ }));
    await user.click(
      await screen.findByRole("option", { name: /Investigate flaky tests/ }),
    );

    expect(
      screen
        .getByLabelText("Task")
        .querySelector('[data-id="session:as-prior"]'),
    ).toHaveTextContent("@Investigate flaky tests");

    await user.click(screen.getByRole("button", { name: "Start session" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({
        task: "refactor the mapper\n\nContext: agent session as-prior",
      }),
    );
  });

  it("rejects more than 32 egress hostnames before ever calling create", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    // Open the Configuration popover, then paste 33 comma-separated hostnames.
    await user.click(screen.getByRole("button", { name: "Configuration" }));
    const egress = await screen.findByLabelText("Egress allowlist");
    const many = Array.from({ length: 33 }, (_, i) => `h${i}.example.com`).join(
      ",",
    );
    fireEvent.change(egress, { target: { value: many } });

    await user.click(screen.getByRole("button", { name: "Start session" }));

    expect(
      await screen.findByText(
        "Too many hostnames — the allowlist allows at most 32 entries.",
      ),
    ).toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();
  });

  it("accepts exactly 32 egress hostnames (boundary) and calls create", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    await user.click(screen.getByRole("button", { name: "Configuration" }));
    const hostnames = Array.from({ length: 32 }, (_, i) => `h${i}.example.com`);
    fireEvent.change(await screen.findByLabelText("Egress allowlist"), {
      target: { value: hostnames.join("\n") },
    });

    await user.click(screen.getByRole("button", { name: "Start session" }));

    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0].egressAllowlist).toEqual(hostnames);
  });

  it("enforces the bex-agent/* branch namespace when the branch is hand-edited", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    await user.click(screen.getByRole("button", { name: "Configuration" }));
    const branch = await screen.findByLabelText("Branch");
    await user.clear(branch);
    await user.type(branch, "main");

    await user.click(screen.getByRole("button", { name: "Start session" }));

    expect(
      await screen.findByText("The branch must be under bex-agent/."),
    ).toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();
  });

  it("renders the house callout when the backend reports the feature unavailable (503)", async () => {
    create.mockRejectedValue(new AgentSessionsUnavailableError());
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    expect(
      await screen.findByText("Agent sessions aren't configured"),
    ).toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("anchors an egress-allowlist code to the egress field and opens Configuration", async () => {
    create.mockRejectedValue(
      new AgentSessionError(
        "AGENT_SESSION_EGRESS_ALLOWLIST_INVALID",
        "server copy",
        { entry: "http://x", reason: "must be a hostname" },
      ),
    );
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    // The i18n-resolved, param-interpolated message anchors to the egress
    // field, and the Configuration popover auto-opens so it's visible.
    expect(
      await screen.findByText(
        'Egress allowlist entry "http://x" is invalid: must be a hostname',
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Egress allowlist")).toBeInTheDocument();
  });

  it("anchors a model-endpoint code to the model endpoint field and opens Configuration", async () => {
    create.mockRejectedValue(
      new AgentSessionError(
        "AGENT_SESSION_MODEL_ENDPOINT_INVALID",
        "server copy",
        {},
      ),
    );
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    expect(
      await screen.findByText("The model endpoint must be a valid HTTPS URL."),
    ).toBeInTheDocument();
    // Configuration auto-opened → the model endpoint input is on screen.
    expect(screen.getByLabelText("Model endpoint")).toBeInTheDocument();
  });

  it("surfaces a non-field-anchored code as a form-level error alert", async () => {
    create.mockRejectedValue(
      new AgentSessionError("AGENT_SESSION_INPUT_INVALID", "server copy", {}),
    );
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    expect(
      await screen.findByText("Couldn't start the session"),
    ).toBeInTheDocument();
    expect(screen.getByText(/That input isn't valid/)).toBeInTheDocument();
  });
});

function agentSession(id: string, task: string): AgentSessionView {
  return {
    id,
    ownerId: "tea-1",
    repo: "acme/widgets",
    branch: `bex-agent/${id}`,
    agentConfig: {
      agent: "claude",
      model: null,
      modelEndpoint: null,
      task,
      template: null,
    },
    sandboxId: null,
    phase: "completed",
    status: "Completed",
    headSha: null,
    prUrl: null,
    prNumber: null,
    evidence: null,
    turns: 1,
    deliveryMode: null,
    failureReason: null,
    createdAt: "2026-08-09T00:00:00Z",
    updatedAt: "2026-08-09T00:00:00Z",
    canceledAt: null,
    isTerminal: true,
    isSteerable: true,
  };
}

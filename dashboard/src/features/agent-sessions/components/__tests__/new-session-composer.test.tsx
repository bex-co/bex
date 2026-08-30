import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NewSessionComposer } from "@/features/agent-sessions/components/new-session-composer";
import {
  AgentSessionError,
  AgentSessionsUnavailableError,
} from "@/features/agent-sessions/lib/errors";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import { agentSessionView } from "@/test/mocks/agent-session";

const mockNavigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
  Link: ({ to, children }: { to: string; children: React.ReactNode }) => (
    <a href={to}>{children}</a>
  ),
}));

const connectGit = vi.fn();
vi.mock("@/features/git/hooks/use-connect-git", () => ({
  useConnectGit: () => ({ connect: connectGit, busy: false }),
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

const create = vi.fn();
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({ create }),
}));

const defaultRepos = [
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
];
const reposState: {
  repos: typeof defaultRepos;
  loading: boolean;
  error: undefined;
} = {
  repos: defaultRepos,
  loading: false,
  error: undefined,
};
vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => reposState,
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
  reposState.repos = defaultRepos;
  reposState.loading = false;
  mockNavigate.mockReset();
  connectGit.mockReset();
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
  // The Tiptap editor mounts after its chunk's dynamic import resolves
  // (lazy-mention-editor), so the first access of a test must await it. The
  // full-suite worker can take longer than Testing Library's 1s default to
  // transform that deliberately split chunk.
  await user.type(
    await screen.findByLabelText("Task", undefined, { timeout: 5_000 }),
    "  refactor the mapper  ",
  );
}

const ATTACHMENT_CONTROL_NAME =
  /^(Add repository(?: or session)?|Mention a repository or session|Repository .+)$/;

function attachmentControl() {
  return screen.getByRole("button", { name: ATTACHMENT_CONTROL_NAME });
}

/**
 * Insert a repo mention through the @ toolbar. The picker is universal now, so
 * the repo is offered directly at the top level — one step, no category hop.
 */
async function pickRepo(
  user: ReturnType<typeof userEvent.setup>,
  match: RegExp = /widgets/,
) {
  await user.click(attachmentControl());
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

    await user.type(await screen.findByLabelText("Task"), "do a thing");
    expect(send).toBeEnabled();
  });

  it("starts a chat-only session when no repo is mentioned", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    // Repo-less sessions are allowed (chat-only): create fires with an empty
    // repo and no derived branch — the agent just runs the prompt and delivers
    // no PR — then navigates like any other session.
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ repo: "", branch: "" }),
    );
  });

  it("opens the toolbar mention after text without requiring manual whitespace", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await user.type(await screen.findByLabelText("Task"), "fix this");

    await user.click(attachmentControl());

    expect(
      await screen.findByRole("option", { name: /Repositories/ }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Task")).toHaveTextContent("fix this @");
  });

  it("opens categories on a typed @, filters, and inserts an atomic inline mention", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    const task = await screen.findByLabelText("Task");

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

  it("surfaces a repo directly on a typed @query and inserts it in one step", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    const task = await screen.findByLabelText("Task");

    // A typed name after @ filters straight to the repo — no @repos: hop.
    await user.type(task, "fix the bug @anv");
    expect(
      await screen.findByRole("option", { name: /anvils/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: /widgets/ }),
    ).not.toBeInTheDocument();
    // The category rows dropped out ("anv" matches neither); the "Repositories"
    // section header (decoration, not an option) groups the match.
    expect(
      screen.queryByRole("option", { name: /^Repositories/ }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Repositories")).toBeInTheDocument();

    // Enter inserts the atomic mention directly from the universal level.
    await user.keyboard("{Enter}");
    expect(task).toHaveTextContent("fix the bug @acme/anvils");
    expect(
      task.querySelector('[data-type="mention"][data-id="repo:acme/anvils"]'),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Start session" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0].repo).toBe("acme/anvils");
  });

  it("previews categories and entities under headers on a bare @", async () => {
    priorSessions = [agentSession("as-prior", "Investigate flaky tests")];
    const user = userEvent.setup();
    render(<NewSessionComposer />);

    await user.type(await screen.findByLabelText("Task"), "scope this @");

    // The category drill-down rows survive for the @repos: shortcut…
    expect(
      await screen.findByRole("option", { name: /^Repositories/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: /^Sessions/ }),
    ).toBeInTheDocument();
    // …and the entities are previewed directly beneath them.
    expect(screen.getByRole("option", { name: /widgets/ })).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: /Investigate flaky tests/ }),
    ).toBeInTheDocument();
  });

  it("surfaces a prior session directly at the universal level", async () => {
    priorSessions = [agentSession("as-prior", "Investigate flaky tests")];
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    const task = await screen.findByLabelText("Task");

    // One step: type a title fragment, pick the session — no @sessions: hop.
    await user.type(task, "look into @flaky");
    await user.click(
      await screen.findByRole("option", { name: /Investigate flaky tests/ }),
    );
    expect(
      task.querySelector('[data-id="session:as-prior"]'),
    ).toBeInTheDocument();

    await pickRepo(user);
    await user.click(screen.getByRole("button", { name: "Start session" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0].task).toContain(
      "Context: agent session as-prior",
    );
  });

  it("does not re-offer an already-mentioned session at the universal level", async () => {
    priorSessions = [agentSession("as-prior", "Investigate flaky tests")];
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    const task = await screen.findByLabelText("Task");

    await user.type(task, "note @flaky");
    await user.click(
      await screen.findByRole("option", { name: /Investigate flaky tests/ }),
    );

    // Reopen the picker — the selected session must not appear again.
    await user.type(task, " @flaky");
    await waitFor(() =>
      expect(
        screen.queryByRole("option", { name: /Investigate flaky tests/ }),
      ).not.toBeInTheDocument(),
    );
  });

  it("replaces the prior inline repo because a session has one checkout", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    const task = await screen.findByLabelText("Task");
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

    await user.click(attachmentControl());
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
    await user.click(screen.getByRole("button", { name: "Advanced" }));
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

    await user.click(screen.getByRole("button", { name: "Advanced" }));
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

    await user.click(screen.getByRole("button", { name: "Advanced" }));
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

  it("lets the toolbar agent select change the create payload without Advanced", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    await user.click(screen.getByLabelText("Agent"));
    await user.click(await screen.findByRole("option", { name: "Gemini" }));
    await user.click(screen.getByRole("button", { name: "Start session" }));

    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0].agent).toBe("gemini");
  });

  it("shows the selected repo on the toolbar chip", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await screen.findByLabelText("Task");
    expect(
      screen.getByRole("button", { name: "Add repository or session" }),
    ).toBeInTheDocument();
    await pickRepo(user);
    expect(
      screen.getByRole("button", { name: "Repository acme/widgets" }),
    ).toBeInTheDocument();
  });

  it("renders exactly one repository/session attachment control", async () => {
    render(<NewSessionComposer />);
    await screen.findByLabelText("Task");

    expect(
      screen.getAllByRole("button", {
        name: ATTACHMENT_CONTROL_NAME,
      }),
    ).toHaveLength(1);
  });

  it("lets chat-only sessions start without a GitHub banner when there are no repos", async () => {
    reposState.repos = [];
    const user = userEvent.setup();
    render(<NewSessionComposer />);

    expect(
      screen.queryByTestId("agent-composer-github-empty"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText("Enter to start · Shift+Enter for a new line"),
    ).toBeInTheDocument();

    await typeTask(user);
    await user.click(screen.getByRole("button", { name: "Start session" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0].repo).toBe("");
  });

  it("inserts a first-run example and opens the mention picker", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await screen.findByLabelText("Task");
    await user.click(
      screen.getByRole("button", {
        name: "Fix the failing tests and open a draft PR",
      }),
    );
    expect(await screen.findByLabelText("Task")).toHaveTextContent(
      "Fix the failing tests and open a draft PR",
    );
    expect(
      await screen.findByRole("option", { name: /Repositories/ }),
    ).toBeInTheDocument();
  });

  it("hides first-run examples once a session exists", () => {
    priorSessions = [agentSession("as-prior", "Investigate flaky tests")];
    render(<NewSessionComposer />);
    expect(
      screen.queryByRole("button", {
        name: "Fix the failing tests and open a draft PR",
      }),
    ).not.toBeInTheDocument();
  });
});

function agentSession(id: string, task: string): AgentSessionView {
  return agentSessionView({ id, task, phase: "completed" });
}

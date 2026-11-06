import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NewSessionComposer } from "@/features/agent-sessions/components/new-session-composer";
import { AgentSessionError } from "@/features/agent-sessions/lib/errors";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import { agentSessionView } from "@/test/mocks/agent-session";
import { agentEnvironment } from "@/test/mocks/agent-environment";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { AgentModelSource } from "@/graphql/definitions";
import type { AgentEnvironmentView } from "@/features/agent-environment/lib/environment";

const mockNavigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
  Link: ({
    to,
    hash,
    children,
  }: {
    to: string;
    hash?: string;
    children: React.ReactNode;
  }) => <a href={hash ? `${to}#${hash}` : to}>{children}</a>,
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

// Uploads resolve only when a test settles them, so the in-flight state is
// observable; tests that never attach are unaffected.
const uploads: { resolve: (v: unknown) => void; file: File }[] = [];
vi.mock("@/features/agent-sessions/lib/attachments", async (orig) => ({
  ...(await orig<typeof import("@/features/agent-sessions/lib/attachments")>()),
  uploadAttachment: (file: File) =>
    new Promise((resolve) => uploads.push({ resolve, file })),
  deleteUnboundAttachment: () => Promise.resolve(),
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

const capabilitiesState = {
  capabilities: {
    enabled: true,
    modelKeyReady: true,
    github: {
      connected: true,
    },
    attachments: { enabled: true, maxBytes: 25 * 1024 * 1024, maxPerTurn: 10 },
  },
  loading: false,
  error: undefined as Error | undefined,
};
vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-capabilities",
  () => ({
    useAgentSessionCapabilities: () => capabilitiesState,
  }),
);

// The workspace agent environment drives the spend banner (ADR091 D6);
// null (not loaded) shows none.
const spend = vi.hoisted(() => ({
  environment: null as AgentEnvironmentView | null,
}));
vi.mock("@/features/agent-environment/hooks/use-agent-environment", () => ({
  useAgentEnvironment: () => ({
    environment: spend.environment,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
    update: vi.fn(),
    saving: false,
  }),
}));

vi.mock("@/features/agent-environment/hooks/use-agent-secrets", () => ({
  useAgentSecrets: () => ({
    names: ["STRIPE_KEY", "SENTRY_DSN"],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
    set: vi.fn(),
    remove: vi.fn(),
    saving: false,
  }),
}));

beforeEach(() => {
  spend.environment = null;
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  uploads.length = 0;
  capabilitiesState.capabilities.attachments.enabled = true;
  priorSessions = [];
  reposState.repos = defaultRepos;
  reposState.loading = false;
  capabilitiesState.capabilities.enabled = true;
  capabilitiesState.capabilities.modelKeyReady = true;
  capabilitiesState.capabilities.github.connected = true;
  capabilitiesState.loading = false;
  capabilitiesState.error = undefined;
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
      }),
    );
    // The model and sandbox network come from the workspace's agent
    // environment (w10/m17); the composer never sends either.
    const input = create.mock.calls[0][0];
    // w21/002: every submit carries a retry-safe idempotency key.
    expect(input.idempotencyKey).toEqual(expect.any(String));
    expect(input.idempotencyKey).not.toBe("");
    expect(input).not.toHaveProperty("model");
    expect(input).not.toHaveProperty("egressAllowlist");
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

  // w4/524: an invalid ceiling disabled Send with an unannounced, unlinked
  // message; the message line is a live region and describes both controls.
  it("describes and announces an invalid spend limit", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await user.type(await screen.findByLabelText("Task"), "do a thing");
    const limit = screen.getByLabelText("Spend limit (USD)");
    const send = screen.getByRole("button", { name: "Start session" });
    const invalid =
      "Enter an amount more than $0 and at most $100,000, or leave it blank.";

    await user.type(limit, "-5");
    expect(limit).toHaveAccessibleDescription(invalid);
    expect(send).toBeDisabled();
    expect(send).toHaveAccessibleDescription(invalid);
    expect(screen.getByRole("status")).toHaveTextContent(invalid);

    await user.clear(limit);
    await user.type(limit, "5");
    expect(limit).not.toHaveAccessibleDescription(invalid);
    expect(send).toBeEnabled();
    expect(send).not.toHaveAccessibleDescription();
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

  it("renders only the task box, the attachment control and the agent select", async () => {
    const { container } = render(<NewSessionComposer />);
    await screen.findByLabelText("Task");

    // No Advanced panel: no branch, model or hostname fields anywhere.
    expect(
      screen.queryByRole("button", { name: "Advanced" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: /branch/i })).toBeNull();
    expect(screen.queryByRole("textbox", { name: /model/i })).toBeNull();
    expect(screen.queryByRole("textbox", { name: /egress|host/i })).toBeNull();
    // The task box is the rich editor; the only plain input is the optional
    // spend limit (w21/005) and no textarea remains (the hidden file input
    // behind the paperclip is not a text field).
    expect(container.querySelectorAll("textarea")).toHaveLength(0);
    expect(
      Array.from(container.querySelectorAll('input:not([type="file"])')).map(
        (el) => el.getAttribute("aria-label"),
      ),
    ).toEqual(["Spend limit (USD)"]);
    expect(screen.getByLabelText("Agent")).toBeInTheDocument();
    expect(attachmentControl()).toBeInTheDocument();
  });

  it("derives a valid bex-agent/* branch even when the task has no ASCII slug", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await user.type(
      await screen.findByLabelText("Task", undefined, { timeout: 5_000 }),
      "修复测试",
    );
    await pickRepo(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0].branch).toMatch(
      /^bex-agent\/session-[0-9a-z]+$/,
    );
  });

  it("sends maxSpendUSD only when a spend limit was typed (w21/005)", async () => {
    const user = userEvent.setup();
    const { unmount } = render(<NewSessionComposer />);
    await typeTask(user);
    await user.click(screen.getByRole("button", { name: "Start session" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0].maxSpendUSD).toBeNull();
    unmount();

    create.mockClear();
    render(<NewSessionComposer />);
    await typeTask(user);
    await user.type(screen.getByLabelText("Spend limit (USD)"), "2.5");
    await user.click(screen.getByRole("button", { name: "Start session" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0].maxSpendUSD).toBe(2.5);
  });

  it("sends picked workspace secrets and inline secrets, then forgets them (ADR098)", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await user.click(screen.getByRole("button", { name: "Secrets" }));
    await user.click(await screen.findByLabelText("STRIPE_KEY"));
    await user.type(screen.getByLabelText("Secret name"), "TEST_TOKEN");
    const value = screen.getByLabelText("Secret value");
    expect(value).toHaveAttribute("type", "password");
    await user.type(value, "tok-123{enter}");
    expect(
      screen.getByTestId("agent-composer-inline-secrets"),
    ).toHaveTextContent("TEST_TOKEN");
    expect(screen.queryByDisplayValue("tok-123")).toBeNull();
    await user.keyboard("{Escape}");
    expect(
      screen.getByRole("button", { name: "Secrets (2 attached)" }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Start session" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0].secretNames).toEqual(["STRIPE_KEY"]);
    expect(create.mock.calls[0][0].secrets).toEqual([
      { name: "TEST_TOKEN", value: "tok-123" },
    ]);
  });

  it("blocks submit on an out-of-range spend limit", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await user.type(screen.getByLabelText("Spend limit (USD)"), "0");
    expect(
      screen.getByRole("button", { name: "Start session" }),
    ).toBeDisabled();
    expect(screen.getByText(/more than \$0 and at most/)).toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();
  });

  it("shows the server's message when the workspace cannot meter a spend limit", async () => {
    create.mockRejectedValue(
      new AgentSessionError(
        "AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE",
        "unavailable",
        {},
      ),
    );
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await user.type(screen.getByLabelText("Spend limit (USD)"), "5");
    await user.click(screen.getByRole("button", { name: "Start session" }));
    expect(
      await screen.findByText(
        /only applies to sessions on the bex-provided model/,
      ),
    ).toBeInTheDocument();
  });

  it("renders the house callout when the backend reports the feature unavailable (503)", async () => {
    create.mockRejectedValue(
      new AgentSessionError(
        "AGENT_SESSION_NOT_CONFIGURED",
        "agent sessions are not configured",
        {},
      ),
    );
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

  it("renders retry copy that is distinct from not-configured for a dependency outage", async () => {
    create.mockRejectedValue(
      new AgentSessionError(
        "AGENT_SESSION_DEPENDENCY_UNAVAILABLE",
        "safe server copy",
        {},
      ),
    );
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    expect(
      await screen.findByText("Agent sessions are temporarily unavailable"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/wait a moment and try again/i),
    ).toBeInTheDocument();
    expect(screen.queryByText(/ask your operator/i)).not.toBeInTheDocument();
  });

  it("reports a missing model key before submit and blocks every session shape", async () => {
    capabilitiesState.capabilities.modelKeyReady = false;
    const user = userEvent.setup();
    render(<NewSessionComposer />);

    expect(
      screen.getByText("Add a model provider key first"),
    ).toBeInTheDocument();
    // The copy names the field that exists: Settings → Agents (ADR091 D7).
    expect(screen.getByText(/under Settings → Agents/)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Open Settings → Agents" }),
    ).toHaveAttribute("href", "/workspace/settings#agents");
    await typeTask(user);
    expect(
      screen.getByRole("button", { name: "Start session" }),
    ).toBeDisabled();
    expect(create).not.toHaveBeenCalled();
  });

  it("allows chat-only without GitHub but explains and blocks a repo-backed session", async () => {
    capabilitiesState.capabilities.github.connected = false;
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);

    // GitHub is irrelevant until a repository is attached.
    expect(
      screen.queryByTestId("agent-composer-github-empty"),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Start session" })).toBeEnabled();

    await pickRepo(user);
    expect(
      screen.getByTestId("agent-composer-github-empty"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Start session" }),
    ).toBeDisabled();
    expect(create).not.toHaveBeenCalled();
  });

  it("surfaces an egress-allowlist code as a form-level error", async () => {
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

    // There is no egress field to anchor to any more; the i18n-resolved,
    // param-interpolated message lands in the form-level alert.
    expect(
      await screen.findByText("Couldn't start the session"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Egress allowlist entry "http://x" is invalid: must be a hostname',
      ),
    ).toBeInTheDocument();
  });

  it("surfaces a model-endpoint code as a form-level error (no endpoint field)", async () => {
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

    // The endpoint is pinned server-side to the selected agent's registered
    // provider, so the composer renders no endpoint field (.pm/w1/079.md);
    // the i18n-resolved message surfaces as the form-level error alert.
    expect(
      await screen.findByText("Couldn't start the session"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("The model endpoint must be a valid HTTPS URL."),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Model endpoint")).not.toBeInTheDocument();
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

  it("lets the toolbar agent select change the create payload", async () => {
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

  // w4/525: the picker's keys arrive through the editor, so once Tab moved
  // focus on, the list stayed open over the next controls and Escape did not
  // close it.
  it("closes the picker when focus leaves the Task box, and on Escape anywhere", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await screen.findByLabelText("Task");

    await user.click(attachmentControl());
    expect(await screen.findByRole("listbox")).toBeInTheDocument();
    await user.click(screen.getByLabelText("Task"));
    await user.tab();
    await waitFor(() =>
      expect(screen.queryByRole("listbox")).not.toBeInTheDocument(),
    );
    expect(screen.getByLabelText("Task")).not.toHaveFocus();

    // Opened again, then Escape while focus is held elsewhere.
    await user.click(attachmentControl());
    expect(await screen.findByRole("listbox")).toBeInTheDocument();
    const outside = document.createElement("div");
    document.body.appendChild(outside);
    outside.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Escape", bubbles: true }),
    );
    await waitFor(() =>
      expect(screen.queryByRole("listbox")).not.toBeInTheDocument(),
    );
    outside.remove();

    // Selecting with the mouse still works: an option click moves no focus.
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

describe("NewSessionComposer attachments (ADR095)", () => {
  it("uploads a picked file and sends its id with the first turn", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await user.upload(
      screen.getByTestId("attachment-input"),
      new File(["boom"], "crash.log", { type: "text/plain" }),
    );

    const send = screen.getByRole("button", { name: "Start session" });
    // The chip is uploading: sending now would drop the file.
    expect(screen.getByTestId("attachment-chip")).toHaveAttribute(
      "data-status",
      "uploading",
    );
    expect(send).toBeDisabled();

    await act(async () =>
      uploads[0].resolve({
        id: "att-1",
        name: "crash.log",
        mediaType: "text/plain",
        size: 4,
      }),
    );
    expect(send).toBeEnabled();
    await user.click(send);

    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ attachmentIds: ["att-1"] }),
    );
    // Sent chips leave the composer; the upload is now bound to the turn.
    await waitFor(() =>
      expect(screen.queryByTestId("attachment-chip")).toBeNull(),
    );
  });

  it("attaches a pasted screenshot instead of pasting it as text", async () => {
    render(<NewSessionComposer />);
    const task = await screen.findByLabelText("Task", undefined, {
      timeout: 5_000,
    });
    const screenshot = new File(["png"], "image.png", { type: "image/png" });
    fireEvent.paste(task, {
      clipboardData: { files: [screenshot], types: ["Files"] },
    });
    await waitFor(() => expect(uploads).toHaveLength(1));
    expect(uploads[0].file).toBe(screenshot);
  });

  it("hides the paperclip until the platform has attachment storage", async () => {
    capabilitiesState.capabilities.attachments.enabled = false;
    render(<NewSessionComposer />);
    await screen.findByLabelText("Task", undefined, { timeout: 5_000 });
    expect(
      screen.queryByRole("button", { name: "Attach files" }),
    ).not.toBeInTheDocument();
  });
});

describe("NewSessionComposer agent model spend (ADR091 D6)", () => {
  function spending(used: number, over: Partial<AgentEnvironmentView> = {}) {
    spend.environment = agentEnvironment({
      agentModelSpendMonthToDateUsd: used,
      agentModelMonthlySpendCapUsd: 20,
      agentModelPlanSpendCapUsd: 20,
      agentModelMaxSpendCapUsd: 200,
      agentModelSpendCapTierLimited: false,
      trustTierLocked: false,
      billingBalanceDue: false,
      ...over,
    });
  }
  const banner = () => screen.queryByTestId("composer-spend-banner");
  const start = () => screen.getByRole("button", { name: "Start session" });

  it("shows no banner below 80% of the cap and lets a session start", async () => {
    spending(15.99);
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    expect(banner()).toBeNull();
    expect(start()).toBeEnabled();
  });

  it("warns at 80% of the cap without blocking", async () => {
    spending(16);
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    expect(banner()).toHaveTextContent(
      "Agent model spend is at 80% of this month's cap",
    );
    expect(banner()).toHaveTextContent("$16.00 of $20.00 used");
    expect(start()).toBeEnabled();
  });

  it("blocks at the cap and offers admins an Increase cap action", async () => {
    spending(20);
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    expect(banner()).toHaveTextContent("Monthly agent model spend cap reached");
    expect(start()).toBeDisabled();
    expect(screen.getByRole("link", { name: "Increase cap" })).toHaveAttribute(
      "href",
      "/workspace/settings#agents",
    );
    await user.keyboard("{Enter}");
    expect(create).not.toHaveBeenCalled();
  });

  it("blocks at the cap without the admin action for non-admins", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    spending(25);
    render(<NewSessionComposer />);
    expect(banner()).toHaveTextContent("Monthly agent model spend cap reached");
    expect(screen.queryByRole("link", { name: "Increase cap" })).toBeNull();
  });

  it("drops the admin action once the cap is at its maximum", () => {
    spending(200, {
      agentModelMonthlySpendCapUsd: 200,
      agentModelSpendCapOverrideUsd: 200,
    });
    render(<NewSessionComposer />);
    expect(banner()).toHaveTextContent("Monthly agent model spend cap reached");
    expect(banner()).toHaveTextContent("the most it can be set to");
    expect(screen.queryByRole("link", { name: "Increase cap" })).toBeNull();
    expect(start()).toBeDisabled();
  });

  it("names the new-workspace limit when the trust tier sets the cap", () => {
    spending(10, {
      agentModelMonthlySpendCapUsd: 10,
      agentModelSpendCapTierLimited: true,
      trustTier: "T0",
    });
    render(<NewSessionComposer />);
    expect(banner()).toHaveTextContent("New-workspace spend limit reached");
    expect(banner()).toHaveTextContent("up to $10.00");
    expect(screen.queryByRole("link", { name: "Increase cap" })).toBeNull();
  });

  it("blocks with a Billing link while a payment is due", async () => {
    spending(1, { billingBalanceDue: true });
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    expect(banner()).toHaveTextContent("A payment failed");
    expect(screen.getByRole("link", { name: "Go to Billing" })).toHaveAttribute(
      "href",
      "/billing",
    );
    expect(start()).toBeDisabled();
  });

  it("shows no spend UI for a workspace on its own model key", async () => {
    spending(500, {
      modelSource: AgentModelSource.Byo,
      modelKeySet: true,
      billingBalanceDue: true,
    });
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    expect(banner()).toBeNull();
    expect(start()).toBeEnabled();
  });

  it("explains the trust tier's concurrent-session limit on create", async () => {
    create.mockRejectedValue(
      new AgentSessionError("AGENT_SESSION_TRUST_LIMIT", "server copy", {
        live: 2,
        limit: 2,
        trustTier: "T0",
        locked: false,
      }),
    );
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await user.click(start());
    expect(
      await screen.findByText(
        /New workspaces can run 2 agent sessions on the bex-provided model at once/,
      ),
    ).toBeInTheDocument();
  });

  it("points a payment-held workspace's session limit at Billing", async () => {
    create.mockRejectedValue(
      new AgentSessionError("AGENT_SESSION_TRUST_LIMIT", "server copy", {
        limit: 2,
        locked: true,
      }),
    );
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await user.click(start());
    expect(
      await screen.findByText(/Settle the outstanding payment in Billing/),
    ).toBeInTheDocument();
  });
});

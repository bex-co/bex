import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NewSessionComposer } from "@/features/agent-sessions/components/new-session-composer";
import {
  AgentSessionError,
  AgentSessionsUnavailableError,
} from "@/features/agent-sessions/lib/errors";

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

vi.mock("@/features/agent-sessions/hooks/use-agent-sessions", () => ({
  useAgentSessions: () => ({
    sessions: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));

beforeEach(() => {
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

/** Set the repo chip through the @ toolbar button → Repositories → item. */
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
  it("submits createAgentSession with the chip's repo, auto-derived branch, and trimmed task, then navigates", async () => {
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

  it("keeps Send disabled until the task text is non-empty", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);

    const send = screen.getByRole("button", { name: "Start session" });
    expect(send).toBeDisabled();

    await user.type(screen.getByLabelText("Task"), "do a thing");
    expect(send).toBeEnabled();
  });

  it("nudges at the @ button instead of submitting when no repo chip is set", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    expect(
      await screen.findByText("Pick a repository with @ first."),
    ).toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();
  });

  it("opens categories on a typed @, inserts the typed token, fuzzy-filters, and embeds a chip on Enter", async () => {
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
    expect(task).toHaveValue("fix the bug @repos:");

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

    // Enter removes the token text and embeds the removable chip.
    await user.keyboard("{Enter}");
    expect(task).toHaveValue("fix the bug ");
    expect(
      screen.getByRole("button", { name: "Remove acme/anvils" }),
    ).toBeInTheDocument();

    // The chip's repo is what the create receives.
    await user.click(screen.getByRole("button", { name: "Start session" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0].repo).toBe("acme/anvils");
  });

  it("re-nudges after the repo chip is removed", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await typeTask(user);
    await pickRepo(user);

    await user.click(
      screen.getByRole("button", { name: "Remove acme/widgets" }),
    );
    await user.click(screen.getByRole("button", { name: "Start session" }));

    expect(
      await screen.findByText("Pick a repository with @ first."),
    ).toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();
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

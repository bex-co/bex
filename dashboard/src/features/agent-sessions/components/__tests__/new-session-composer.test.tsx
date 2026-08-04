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

/** Fill the always-visible required fields with a valid task + repo. */
async function fillRequired(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("Task"), "  refactor the mapper  ");
  await user.type(screen.getByLabelText("Repository"), " acme/widgets ");
}

describe("NewSessionComposer", () => {
  it("submits createAgentSession with trimmed values and navigates to the new session", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await fillRequired(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({
        ownerId: "tea-1",
        repo: "acme/widgets",
        branch: "bex-agent/",
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

  it("blocks submit and shows validation when required fields are empty", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);

    // Clear the pre-filled branch too, so all three required rules fire.
    await user.clear(screen.getByLabelText("Branch"));
    await user.click(screen.getByRole("button", { name: "Start session" }));

    expect(
      await screen.findByText("Describe the task for the agent."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Enter a repository as owner/name."),
    ).toBeInTheDocument();
    expect(screen.getByText("Enter a working branch.")).toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();
  });

  it("rejects more than 32 egress hostnames before ever calling create", async () => {
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await fillRequired(user);

    // Reveal the Advanced section, then paste 33 comma-separated hostnames.
    await user.click(screen.getByRole("button", { name: "Advanced" }));
    const egress = screen.getByLabelText("Egress allowlist");
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
    await fillRequired(user);

    await user.click(screen.getByRole("button", { name: "Advanced" }));
    const hostnames = Array.from({ length: 32 }, (_, i) => `h${i}.example.com`);
    fireEvent.change(screen.getByLabelText("Egress allowlist"), {
      target: { value: hostnames.join("\n") },
    });

    await user.click(screen.getByRole("button", { name: "Start session" }));

    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0].egressAllowlist).toEqual(hostnames);
  });

  it("renders the house callout when the backend reports the feature unavailable (503)", async () => {
    create.mockRejectedValue(new AgentSessionsUnavailableError());
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await fillRequired(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    expect(
      await screen.findByText("Agent sessions aren't configured"),
    ).toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("anchors an egress-allowlist code to the egress field and opens Advanced", async () => {
    create.mockRejectedValue(
      new AgentSessionError(
        "AGENT_SESSION_EGRESS_ALLOWLIST_INVALID",
        "server copy",
        { entry: "http://x", reason: "must be a hostname" },
      ),
    );
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await fillRequired(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    // The i18n-resolved, param-interpolated message anchors to the egress field,
    // and the Advanced section auto-expands so the field is visible.
    expect(
      await screen.findByText(
        'Egress allowlist entry "http://x" is invalid: must be a hostname',
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Egress allowlist")).toBeInTheDocument();
  });

  it("anchors a model-endpoint code to the model endpoint field", async () => {
    create.mockRejectedValue(
      new AgentSessionError(
        "AGENT_SESSION_MODEL_ENDPOINT_INVALID",
        "server copy",
        {},
      ),
    );
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await fillRequired(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    expect(
      await screen.findByText("The model endpoint must be a valid HTTPS URL."),
    ).toBeInTheDocument();
    // Advanced opened → the model endpoint input is on screen.
    expect(screen.getByLabelText("Model endpoint")).toBeInTheDocument();
  });

  it("surfaces a non-field-anchored code as a form-level error alert", async () => {
    create.mockRejectedValue(
      new AgentSessionError("AGENT_SESSION_INPUT_INVALID", "server copy", {}),
    );
    const user = userEvent.setup();
    render(<NewSessionComposer />);
    await fillRequired(user);

    await user.click(screen.getByRole("button", { name: "Start session" }));

    expect(
      await screen.findByText("Couldn't start the session"),
    ).toBeInTheDocument();
    expect(screen.getByText(/That input isn't valid/)).toBeInTheDocument();
  });
});

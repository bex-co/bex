import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { SessionDetailHeader } from "@/features/agent-sessions/components/session-detail-header";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import type { HasSSHKeyState } from "@/features/ssh-keys/hooks/use-has-ssh-key";
import { agentSessionView } from "@/test/mocks/agent-session";

// The header's back link uses the router; the Connect control doesn't, so a
// stub Link keeps the render provider-free.
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
  useNavigate: () => vi.fn(),
}));

// Drives the RequiresSshKey gate: default has-a-key so the existing zed-link
// assertions hold; the no-key case overrides it.
const hasKeyState: HasSSHKeyState = {
  hasKey: true,
  loading: false,
  error: false,
};
vi.mock("@/features/ssh-keys/hooks/use-has-ssh-key", () => ({
  useHasSSHKey: () => hasKeyState,
}));

const apollo = new ApolloClient({
  cache: new InMemoryCache(),
  link: ApolloLink.empty(),
});

beforeEach(() => {
  hasKeyState.hasKey = true;
  hasKeyState.loading = false;
  hasKeyState.error = false;
  ssh.enabled = true;
  resume.mockReset();
});

// The header calls the cancel mutation hook (Apollo); Open in Zed calls
// resume to start a closed session's sandbox (ADR054 D8). Stubs keep the test
// off the network.
const resume = vi.hoisted(() => vi.fn());
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({
    cancel: vi.fn(),
    archive: vi.fn(),
    unarchive: vi.fn(),
    deleteSession: vi.fn(),
    resume,
  }),
}));

// Whether the deployment serves editors over SSH (capabilities.sshEnabled).
const ssh = vi.hoisted(() => ({ enabled: true }));
vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-capabilities",
  () => ({
    useAgentSessionCapabilities: () => ({
      capabilities: { sshEnabled: ssh.enabled, sandbox: null },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    }),
  }),
);

function view(over: Partial<AgentSessionView> = {}): AgentSessionView {
  return agentSessionView({
    id: "ags-1",
    task: "do the thing",
    sandboxId: "os-x",
    sshAddress: "ags-1@ssh.bex.co",
    status: "working",
    turns: 1,
    ...over,
  });
}

function renderHeader(session: AgentSessionView) {
  const tree = (s: AgentSessionView) => (
    <ApolloProvider client={apollo}>
      <SessionDetailHeader session={s} />
    </ApolloProvider>
  );
  const result = render(tree(session));
  return {
    ...result,
    update: (s: AgentSessionView) => result.rerender(tree(s)),
  };
}

// A completed session whose sandbox closed at its idle timeout.
function closed(over: Partial<AgentSessionView> = {}): AgentSessionView {
  return view({
    phase: "completed",
    sandboxId: null,
    sshAddress: null,
    status: "completed",
    ...over,
  });
}

describe("Open in Zed", () => {
  it("offers a zed:// hotlink and a copyable ssh command when the caller has a key", async () => {
    renderHeader(view());
    await userEvent.click(screen.getByRole("button", { name: /connect/i }));

    const zed = screen.getByRole("menuitem", { name: /open in zed/i });
    expect(zed).toHaveAttribute("href", "zed://ssh/ags-1@ssh.bex.co/workspace");

    expect(screen.getByText("ssh ags-1@ssh.bex.co")).toBeInTheDocument();
  });

  it("swaps the doomed payload for an add-key CTA when the caller has no key", async () => {
    hasKeyState.hasKey = false;
    renderHeader(view());
    await userEvent.click(screen.getByRole("button", { name: /connect/i }));

    // The CTA replaces the zed:// link and the ssh command — no dead-end action.
    expect(screen.getByText("Add an SSH key to connect")).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: /open in zed/i }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("ssh ags-1@ssh.bex.co")).not.toBeInTheDocument();
  });

  it("hides the whole control when the backend surfaces no address", () => {
    renderHeader(view({ sshAddress: null }));
    expect(
      screen.queryByRole("button", { name: /connect/i }),
    ).not.toBeInTheDocument();
  });
});

describe("Open in Zed on a closed session (ADR054 D8)", () => {
  it("starts the sandbox, says it is starting, then offers the link once it is up", async () => {
    resume.mockResolvedValue({});
    const { update } = renderHeader(closed());
    await userEvent.click(screen.getByRole("button", { name: /connect/i }));
    expect(
      screen.getByText(/The sandbox closed after the session sat idle/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: /open in zed/i }),
    ).not.toBeInTheDocument();

    await userEvent.click(
      screen.getByRole("button", { name: "Start the sandbox" }),
    );
    expect(resume).toHaveBeenCalledWith("ags-1");

    // The session reads resuming while the sandbox starts.
    update(closed({ phase: "resuming", status: "resuming" }));
    expect(screen.getByRole("status")).toHaveTextContent(
      "Starting the sandbox",
    );
    expect(
      screen.queryByRole("button", { name: "Start the sandbox" }),
    ).not.toBeInTheDocument();

    // Up: completed again, with its sandbox and address.
    update(view({ phase: "completed", status: "completed" }));
    expect(
      screen.getByRole("menuitem", { name: /open in zed/i }),
    ).toHaveAttribute("href", "zed://ssh/ags-1@ssh.bex.co/workspace");
    expect(screen.getByText("ssh ags-1@ssh.bex.co")).toBeInTheDocument();

    // A later idle close offers the start again, not a failure.
    update(closed());
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Start the sandbox" }),
    ).toBeInTheDocument();
  });

  it("shows a refused start and lets the person try again", async () => {
    resume.mockRejectedValue(
      new Error("this workspace already has 2 live agent-session sandboxes"),
    );
    renderHeader(closed());
    await userEvent.click(screen.getByRole("button", { name: /connect/i }));
    await userEvent.click(
      screen.getByRole("button", { name: "Start the sandbox" }),
    );
    await waitFor(() => expect(screen.getByRole("alert")).toBeInTheDocument());
    expect(
      screen.getByRole("button", { name: "Start the sandbox" }),
    ).toBeEnabled();
  });

  it("says so when a start ended without a sandbox", async () => {
    resume.mockResolvedValue({});
    const { update } = renderHeader(closed());
    await userEvent.click(screen.getByRole("button", { name: /connect/i }));
    await userEvent.click(
      screen.getByRole("button", { name: "Start the sandbox" }),
    );
    update(closed({ phase: "resuming", status: "resuming" }));
    update(closed());
    expect(screen.getByRole("alert")).toHaveTextContent(
      "The sandbox did not start. Try again.",
    );
  });

  it("offers nothing where the deployment serves no SSH, or for an archived or canceled session", () => {
    ssh.enabled = false;
    const { update } = renderHeader(closed());
    expect(
      screen.queryByRole("button", { name: /connect/i }),
    ).not.toBeInTheDocument();
    ssh.enabled = true;
    update(
      closed({
        archivedAt: "2026-11-22T00:00:00Z",
        isArchived: true,
        isSteerable: false,
      }),
    );
    expect(
      screen.queryByRole("button", { name: /connect/i }),
    ).not.toBeInTheDocument();
    update(closed({ phase: "canceled", isSteerable: false, isTerminal: true }));
    expect(
      screen.queryByRole("button", { name: /connect/i }),
    ).not.toBeInTheDocument();
  });

  it("asks for an SSH key before starting anything", async () => {
    hasKeyState.hasKey = false;
    renderHeader(closed());
    await userEvent.click(screen.getByRole("button", { name: /connect/i }));
    expect(screen.getByText("Add an SSH key to connect")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Start the sandbox" }),
    ).not.toBeInTheDocument();
  });
});

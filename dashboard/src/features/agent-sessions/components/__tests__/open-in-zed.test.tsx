import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
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
});

// The header calls the cancel mutation hook (Apollo); a stub keeps the test
// off the network. Open in Zed calls none: connecting wakes a closed sandbox
// (ADR054 D9).
const mutations = vi.hoisted(() => ({ resume: vi.fn() }));
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({
    cancel: vi.fn(),
    archive: vi.fn(),
    unarchive: vi.fn(),
    deleteSession: vi.fn(),
    resume: mutations.resume,
  }),
}));

vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-capabilities",
  () => ({
    useAgentSessionCapabilities: () => ({
      capabilities: { sshEnabled: true, sandbox: null },
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

// A completed session whose sandbox closed at its idle timeout: the backend
// still surfaces its address, because connecting wakes it (w10/m597).
function closed(over: Partial<AgentSessionView> = {}): AgentSessionView {
  return view({
    phase: "completed",
    sandboxId: null,
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

describe("Open in Zed on a closed session (ADR054 D9)", () => {
  it("offers the link and the ssh command at once, and starts nothing itself", async () => {
    renderHeader(closed());
    await userEvent.click(screen.getByRole("button", { name: /connect/i }));
    expect(
      screen.getByRole("menuitem", { name: /open in zed/i }),
    ).toHaveAttribute("href", "zed://ssh/ags-1@ssh.bex.co/workspace");
    expect(screen.getByText("ssh ags-1@ssh.bex.co")).toBeInTheDocument();
    expect(
      screen.getByText(/A sandbox that closed starts when you connect/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Start the sandbox" }),
    ).not.toBeInTheDocument();
    expect(mutations.resume).not.toHaveBeenCalled();
  });

  it("keeps the targets while a sandbox starts, the trigger showing it is busy", async () => {
    renderHeader(closed({ phase: "resuming", status: "working" }));
    const trigger = screen.getByRole("button", { name: /connect/i });
    expect(trigger.querySelector(".animate-spin")).not.toBeNull();
    await userEvent.click(trigger);
    expect(screen.getByText("ssh ags-1@ssh.bex.co")).toBeInTheDocument();
  });

  it("offers nothing where the backend gives no address: no SSH here, or an archived or canceled session", () => {
    renderHeader(
      closed({
        sshAddress: null,
        archivedAt: "2026-11-22T00:00:00Z",
        isArchived: true,
        isSteerable: false,
      }),
    );
    expect(
      screen.queryByRole("button", { name: /connect/i }),
    ).not.toBeInTheDocument();
  });

  it("asks for an SSH key before offering anything", async () => {
    hasKeyState.hasKey = false;
    renderHeader(closed());
    await userEvent.click(screen.getByRole("button", { name: /connect/i }));
    expect(screen.getByText("Add an SSH key to connect")).toBeInTheDocument();
    expect(screen.queryByText("ssh ags-1@ssh.bex.co")).not.toBeInTheDocument();
  });
});

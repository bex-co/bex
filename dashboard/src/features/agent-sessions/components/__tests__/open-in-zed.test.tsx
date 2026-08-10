import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { SessionDetailHeader } from "@/features/agent-sessions/components/session-detail-header";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import type { HasSSHKeyState } from "@/features/ssh-keys/hooks/use-has-ssh-key";

// The header's back link uses the router; the Connect control doesn't, so a
// stub Link keeps the render provider-free.
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
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

// The header calls the cancel mutation hook (Apollo); the Open-in-Zed control
// doesn't touch it, so a stub keeps the test off the network.
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({ cancel: vi.fn() }),
}));

function view(over: Partial<AgentSessionView> = {}): AgentSessionView {
  return {
    id: "ags-1",
    ownerId: "tea-1",
    repo: "acme/widgets",
    branch: "bex-agent/fix",
    agentConfig: {
      agent: "claude",
      model: null,
      modelEndpoint: null,
      task: "do the thing",
      template: null,
    },
    sandboxId: "os-x",
    sshAddress: "ags-1@ssh.bex.co",
    phase: "running",
    status: "working",
    headSha: null,
    prUrl: null,
    prNumber: null,
    evidence: null,
    turns: 1,
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

function renderHeader(session: AgentSessionView) {
  return render(
    <ApolloProvider client={apollo}>
      <SessionDetailHeader session={session} />
    </ApolloProvider>,
  );
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
    expect(
      screen.getByText("Add an SSH key to connect"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: /open in zed/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("ssh ags-1@ssh.bex.co"),
    ).not.toBeInTheDocument();
  });

  it("hides the whole control when the backend surfaces no address", () => {
    renderHeader(view({ sshAddress: null }));
    expect(
      screen.queryByRole("button", { name: /connect/i }),
    ).not.toBeInTheDocument();
  });
});

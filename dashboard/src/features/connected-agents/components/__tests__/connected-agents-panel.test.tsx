import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConnectedAgentsPanel } from "@/features/connected-agents/components/connected-agents-panel";
import type { ConnectedAgentView } from "@/features/connected-agents/types";

const agentsState: {
  agents: ConnectedAgentView[];
  loading: boolean;
  error: boolean;
} = { agents: [], loading: false, error: false };
const revoke = vi.fn();
vi.mock("@/features/connected-agents/hooks/use-connected-agents", () => ({
  useConnectedAgents: () => ({
    ...agentsState,
    revoke,
    revoking: null,
    refetch: vi.fn(),
  }),
}));

beforeEach(() => {
  agentsState.agents = [];
  agentsState.loading = false;
  agentsState.error = false;
  revoke.mockReset();
});

describe("ConnectedAgentsPanel", () => {
  it("lists every client the user has authorized, with scopes and grant date", () => {
    agentsState.agents = [
      {
        clientId: "agent-1",
        clientName: "Claude Code",
        scopes: ["openid", "offline_access"],
        grantedAt: "2026-01-01T00:00:00.000Z",
      },
    ];
    render(<ConnectedAgentsPanel />);

    expect(screen.getByText("Claude Code")).toBeInTheDocument();
    expect(screen.getByText("openid")).toBeInTheDocument();
    expect(screen.getByText("offline_access")).toBeInTheDocument();
  });

  it("shows an empty state with no connected agents", () => {
    render(<ConnectedAgentsPanel />);
    expect(screen.getByText("No connected agents")).toBeInTheDocument();
  });

  it("shows a generic error state on failure", () => {
    agentsState.error = true;
    render(<ConnectedAgentsPanel />);
    expect(
      screen.getByText("Couldn't load connected agents"),
    ).toBeInTheDocument();
  });

  it("confirming the dialog calls revoke with the client's id and name", async () => {
    agentsState.agents = [
      {
        clientId: "agent-1",
        clientName: "Claude Code",
        scopes: [],
        grantedAt: null,
      },
    ];
    revoke.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<ConnectedAgentsPanel />);

    await user.click(screen.getByRole("button", { name: "Revoke" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getAllByRole("button", { name: "Revoke" })[0],
    );

    expect(revoke).toHaveBeenCalledWith("agent-1", "Claude Code");
  });
});

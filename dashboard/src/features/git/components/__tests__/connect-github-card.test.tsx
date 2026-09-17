import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConnectGithubCard } from "@/features/git/components/connect-github-card";

const refetch = vi.fn();
vi.mock("@/features/git/hooks/use-git-connection", () => ({
  useGitConnections: () => ({
    connections: [],
    connected: false,
    loading: false,
    error: undefined,
    refetch,
  }),
}));

vi.mock("@/features/git/hooks/use-connect-git", () => ({
  useConnectGit: () => ({ connect: vi.fn(), busy: false }),
}));

vi.mock("@/features/git/hooks/use-claim-git", () => ({
  useClaimGit: () => ({ claim: vi.fn(), busy: false }),
}));

vi.mock("@/features/git/hooks/use-disconnect-git", () => ({
  useDisconnectGit: () => ({ disconnect: vi.fn(), busy: false }),
}));

const select = vi.fn();
const selectionState = {
  candidates: [] as { installationId: number; accountLogin: string }[],
  loading: false,
  gone: false,
  selecting: false,
  select,
};
vi.mock("@/features/git/hooks/use-claim-selection", () => ({
  useClaimSelection: (id?: string) =>
    id ? selectionState : { ...selectionState, candidates: [] },
}));

beforeEach(() => {
  refetch.mockReset();
  select.mockReset();
  select.mockResolvedValue(true);
  selectionState.candidates = [];
  selectionState.loading = false;
  selectionState.gone = false;
  selectionState.selecting = false;
});

describe("ConnectGithubCard callback failures", () => {
  it("shows an expired callback as a visible retryable error", () => {
    render(<ConnectGithubCard callbackError="expired_state" />);

    expect(
      screen.getByText("GitHub connection wasn't completed"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "This connection request expired. Select Connect GitHub to try again.",
      ),
    ).toBeInTheDocument();
  });

  it("does not reflect an unknown callback value", () => {
    render(<ConnectGithubCard callbackError="attacker-controlled-text" />);

    expect(
      screen.getByText(
        "GitHub couldn't complete the connection. Select Connect GitHub to try again.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("attacker-controlled-text"),
    ).not.toBeInTheDocument();
  });
});

// w2/m162 (ADR078 §2/§3a): after N:N, no message may tell a user to install an
// App they have installed or to uninstall one — both describe states that can no
// longer arise. These two strings were the reported dead end's voice.
describe("ConnectGithubCard post-N:N copy", () => {
  it("never tells the user to uninstall an installation", () => {
    render(<ConnectGithubCard callbackError="ambiguous_installation" />);

    expect(document.body.textContent).not.toMatch(/uninstall/i);
  });

  it("offers the claim recovery rather than a bare failure", () => {
    render(<ConnectGithubCard callbackError="no_claimable_installation" />);

    expect(
      screen.getByText(
        "No GitHub account you administer was found. Check that you authorized the right GitHub user, or install the bex GitHub App on the account first.",
      ),
    ).toBeInTheDocument();
  });

  it("warns that the install path never returns for an already-installed account", () => {
    render(<ConnectGithubCard />);

    expect(document.body.textContent).toContain(
      "GitHub shows Configure rather than Install",
    );
  });
});

describe("ConnectGithubCard claim account picker", () => {
  it("renders one option per proved candidate and binds the chosen one", async () => {
    const user = userEvent.setup();
    selectionState.candidates = [
      { installationId: 1, accountLogin: "octo-org" },
      { installationId: 2, accountLogin: "puncsky" },
    ];

    render(<ConnectGithubCard claimSelectionId="gcs-1" />);

    expect(
      screen.getByText("Choose a GitHub account to connect"),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /puncsky/ }));

    expect(select).toHaveBeenCalledWith(2);
    await waitFor(() => expect(refetch).toHaveBeenCalled());
  });

  it("does not render the picker without a selection id", () => {
    selectionState.candidates = [
      { installationId: 1, accountLogin: "octo-org" },
    ];

    render(<ConnectGithubCard />);

    expect(
      screen.queryByText("Choose a GitHub account to connect"),
    ).not.toBeInTheDocument();
  });

  it("offers a restart when the selection is expired or already spent", () => {
    selectionState.gone = true;

    render(<ConnectGithubCard claimSelectionId="gcs-stale" />);

    expect(screen.getByText("This choice has expired")).toBeInTheDocument();
    expect(
      screen.getByText("Select Claim installed account below to start again."),
    ).toBeInTheDocument();
  });
});

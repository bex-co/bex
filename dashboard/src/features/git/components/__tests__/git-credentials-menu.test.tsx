import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GitCredentialsMenu } from "@/features/git/components/git-credentials-menu";

interface Conn {
  accountLogin: string;
  installationId: number;
  createdAt: string;
  installUrl: string;
}

const state: {
  connections: Conn[];
  connected: boolean;
  loading: boolean;
  error: Error | undefined;
  repos: { accountLogin: string }[];
} = {
  connections: [],
  connected: false,
  loading: false,
  error: undefined,
  repos: [],
};

const refetch = vi.fn();
const refetchRepos = vi.fn();
const connect = vi.fn();
const claim = vi.fn();
const disconnect = vi.fn().mockResolvedValue(true);

vi.mock("@/features/git/hooks/use-git-connection", () => ({
  useGitConnections: () => ({
    connections: state.connections,
    connected: state.connected,
    loading: state.loading,
    error: state.error,
    refetch,
  }),
}));
vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => ({
    repos: state.repos,
    loading: false,
    error: undefined,
    refetch: refetchRepos,
  }),
}));
vi.mock("@/features/git/hooks/use-connect-git", () => ({
  useConnectGit: () => ({ connect, busy: false }),
}));
vi.mock("@/features/git/hooks/use-claim-git", () => ({
  useClaimGit: () => ({ claim, busy: false }),
}));
vi.mock("@/features/git/hooks/use-disconnect-git", () => ({
  useDisconnectGit: () => ({ disconnect, busy: false }),
}));

const CONNECTED: Conn[] = [
  {
    accountLogin: "puncsky",
    installationId: 111,
    createdAt: "2026-08-01",
    installUrl: "https://github.com/settings/installations/111",
  },
  {
    accountLogin: "bex-co",
    installationId: 222,
    createdAt: "2026-08-02",
    installUrl: "https://github.com/settings/installations/222",
  },
];

function connectedState() {
  state.connections = CONNECTED;
  state.connected = true;
  state.repos = [
    { accountLogin: "puncsky" },
    { accountLogin: "puncsky" },
    { accountLogin: "puncsky" },
    { accountLogin: "bex-co" },
  ];
}

beforeEach(() => {
  state.connections = [];
  state.connected = false;
  state.loading = false;
  state.error = undefined;
  state.repos = [];
  refetch.mockReset();
  refetchRepos.mockReset();
  connect.mockReset();
  claim.mockReset();
  disconnect.mockClear();
});

describe("GitCredentialsMenu", () => {
  it("shows the connected-account count in the trigger", () => {
    connectedState();
    render(<GitCredentialsMenu />);
    expect(
      screen.getByRole("button", { name: /Credentials \(2\)/ }),
    ).toBeInTheDocument();
  });

  it("lists each account with its repo count, Open-in-GitHub, and Configure links", async () => {
    connectedState();
    const user = userEvent.setup();
    render(<GitCredentialsMenu />);
    await user.click(screen.getByRole("button", { name: /Credentials/ }));

    // Account link points at the account's GitHub page.
    const puncsky = screen.getByRole("link", { name: /puncsky/ });
    expect(puncsky).toHaveAttribute("href", "https://github.com/puncsky");
    // Repo counts, grouped by accountLogin.
    expect(screen.getByText("3 repos")).toBeInTheDocument();
    expect(screen.getByText("1 repos")).toBeInTheDocument();
    // Configure-in-GitHub links carry each installation's grants URL.
    const configure = screen.getAllByLabelText("Configure in GitHub");
    expect(configure).toHaveLength(2);
    expect(configure[0]).toHaveAttribute(
      "href",
      "https://github.com/settings/installations/111",
    );
  });

  it("disconnects a specific account after confirmation", async () => {
    connectedState();
    const user = userEvent.setup();
    render(<GitCredentialsMenu />);
    await user.click(screen.getByRole("button", { name: /Credentials/ }));
    await user.click(
      screen.getByRole("button", { name: "Disconnect puncsky" }),
    );
    // The confirm dialog's confirm button has the exact name "Disconnect".
    await user.click(screen.getByRole("button", { name: "Disconnect" }));

    expect(disconnect).toHaveBeenCalledWith(111);
  });

  it("connects another account and claims from the connected state", async () => {
    connectedState();
    const user = userEvent.setup();
    render(<GitCredentialsMenu />);
    await user.click(screen.getByRole("button", { name: /Credentials/ }));

    await user.click(
      screen.getByRole("button", { name: "Connect another account" }),
    );
    expect(connect).toHaveBeenCalledTimes(1);

    await user.click(
      screen.getByRole("button", { name: "Claim installed account" }),
    );
    expect(claim).toHaveBeenCalledTimes(1);
  });

  it("offers Connect in the disconnected state", async () => {
    const user = userEvent.setup();
    render(<GitCredentialsMenu />);
    expect(
      screen.getByRole("button", { name: /Credentials \(0\)/ }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Credentials/ }));
    await user.click(screen.getByRole("button", { name: "Connect GitHub" }));
    expect(connect).toHaveBeenCalledTimes(1);
  });

  it("shows the unavailable state when the GitHub App is not configured", async () => {
    state.error = new Error("github integration not configured");
    const user = userEvent.setup();
    render(<GitCredentialsMenu />);
    await user.click(screen.getByRole("button", { name: /Credentials/ }));
    expect(
      screen.getByText("GitHub integration not configured"),
    ).toBeInTheDocument();
  });

  it("shows a generic error state on a load failure", async () => {
    state.error = new Error("boom");
    const user = userEvent.setup();
    render(<GitCredentialsMenu />);
    await user.click(screen.getByRole("button", { name: /Credentials/ }));
    expect(
      screen.getByText("Couldn't load the GitHub connection"),
    ).toBeInTheDocument();
  });
});

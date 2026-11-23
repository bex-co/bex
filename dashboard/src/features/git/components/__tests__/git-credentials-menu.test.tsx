import { beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";
import { render, screen, waitFor } from "@testing-library/react";
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
// connect is the single connect entry (chooser mode, ADR078 §3b); install is
// the install-first flow the menu must never start on its own.
const connect = vi.fn();
const install = vi.fn();
const connectOptions = vi.fn();
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
  useConnectGitAccount: (options: unknown) => {
    connectOptions(options);
    return { connect, busy: false };
  },
  useConnectGit: () => ({ connect: install, busy: false }),
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
  install.mockReset();
  connectOptions.mockReset();
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

    // w4/607: the panel was an unnamed dialog.
    expect(
      screen.getByRole("dialog", { name: "Credentials" }),
    ).toBeInTheDocument();
    // Account link points at the account's GitHub page.
    const puncsky = screen.getByRole("link", { name: /puncsky/ });
    expect(puncsky).toHaveAttribute("href", "https://github.com/puncsky");
    // Repo counts, grouped by accountLogin.
    expect(screen.getByText("3 repos")).toBeInTheDocument();
    expect(screen.getByText("1 repo")).toBeInTheDocument();
    // Configure-in-GitHub links carry each installation's grants URL. Each is
    // named by its own visible words: as a bare glyph it said nothing.
    const configure = screen.getAllByRole("link", {
      name: "Configure in GitHub",
    });
    expect(configure).toHaveLength(2);
    expect(configure[0]).toHaveTextContent("Configure in GitHub");
    // So is Disconnect, named for its account to a screen reader.
    expect(
      screen.getByRole("button", { name: "Disconnect puncsky" }),
    ).toHaveTextContent("Disconnect");
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

  // w1/m526 (ADR078 §3b): one connect action, in a new tab so the form behind
  // the menu survives, and never the install-first flow or a second "claim".
  it("offers one connect action from the connected state, in a new tab", async () => {
    connectedState();
    const user = userEvent.setup();
    render(<GitCredentialsMenu />);
    await user.click(screen.getByRole("button", { name: /Credentials/ }));

    await user.click(
      screen.getByRole("button", { name: "Connect another account" }),
    );
    expect(connect).toHaveBeenCalledTimes(1);
    expect(install).not.toHaveBeenCalled();
    expect(connectOptions).toHaveBeenCalledWith({ newTab: true });
    expect(screen.queryByText(/claim/i)).toBeNull();
  });

  it("offers Connect GitHub account in the disconnected state", async () => {
    const user = userEvent.setup();
    render(<GitCredentialsMenu />);
    expect(
      screen.getByRole("button", { name: /Credentials \(0\)/ }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Credentials/ }));
    await user.click(
      screen.getByRole("button", { name: "Connect GitHub account" }),
    );
    expect(connect).toHaveBeenCalledTimes(1);
    expect(install).not.toHaveBeenCalled();
  });

  // w1/m526 Finding 4 (DoD 6): Radix skips links when it auto-focuses, and each
  // row's account name and Configure entry are links, so focus used to land on
  // "Disconnect bex-co".
  it("opens with focus on the connect action, never on a Disconnect button", async () => {
    connectedState();
    const user = userEvent.setup();
    render(<GitCredentialsMenu />);
    await user.click(screen.getByRole("button", { name: /Credentials/ }));

    const panel = screen.getByRole("dialog", { name: "Credentials" });
    await waitFor(() =>
      expect(panel).toContainElement(document.activeElement as HTMLElement),
    );
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "Connect another account" }),
    );
    for (const name of ["Disconnect puncsky", "Disconnect bex-co"]) {
      expect(screen.getByRole("button", { name })).not.toHaveFocus();
    }
  });

  // The trigger shows beside the repository search before the accounts are
  // read. "Credentials (0)" there told a connected workspace it had none.
  it("shows no count until the accounts are read", () => {
    state.loading = true;
    const { rerender } = render(<GitCredentialsMenu />);
    const trigger = screen.getByRole("button", { name: /Credentials/ });
    expect(trigger).toHaveTextContent(/^Credentials$/);
    expect(trigger.querySelector('[data-slot="skeleton"]')).not.toBeNull();

    state.loading = false;
    state.connected = true;
    state.connections = [
      {
        accountLogin: "acme",
        installationId: 42,
        createdAt: "",
        installUrl: "https://github.com/settings/installations/42",
      },
    ];
    rerender(<GitCredentialsMenu />);
    expect(trigger).toHaveTextContent("Credentials (1)");
    expect(trigger.querySelector('[data-slot="skeleton"]')).toBeNull();
  });

  it("focuses the panel itself while it offers no action", async () => {
    state.loading = true;
    const user = userEvent.setup();
    render(<GitCredentialsMenu />);
    await user.click(screen.getByRole("button", { name: /Credentials/ }));

    const panel = screen.getByRole("dialog", { name: "Credentials" });
    await waitFor(() => expect(panel).toHaveFocus());
  });

  it("shows the unavailable state when the GitHub App is not configured", async () => {
    state.error = codedGraphQLError(
      "GITHUB_UNAVAILABLE",
      {},
      "github integration not configured",
    );
    const user = userEvent.setup();
    render(<GitCredentialsMenu />);
    await user.click(screen.getByRole("button", { name: /Credentials/ }));
    expect(
      screen.getByText("GitHub integration not configured"),
    ).toBeInTheDocument();
  });

  // GitHub is recognized as unconfigured by GITHUB_UNAVAILABLE, not by "not
  // configured" in the wording (w5/m130).
  it("does not read the unavailable state from the wording alone", async () => {
    state.error = uncodedGraphQLError("github integration not configured");
    const user = userEvent.setup();
    render(<GitCredentialsMenu />);
    await user.click(screen.getByRole("button", { name: /Credentials/ }));
    expect(
      screen.queryByText("GitHub integration not configured"),
    ).not.toBeInTheDocument();
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

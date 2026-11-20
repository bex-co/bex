import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConnectGithubCard } from "@/features/git/components/connect-github-card";
import { hydrateAcrossBoundary } from "@/test/hydration";

const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
}));

const toastSuccess = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: (...a: unknown[]) => toastSuccess(...a), error: vi.fn() },
}));

interface Conn {
  accountLogin: string;
  installationId: number;
  createdAt: string;
  installUrl: string;
}
const refetch = vi.fn();
const gitState = { loading: false, connections: [] as Conn[] };
vi.mock("@/features/git/hooks/use-git-connection", () => ({
  useGitConnections: () => ({
    connections: gitState.connections,
    connected: gitState.connections.length > 0,
    loading: gitState.loading,
    error: undefined,
    refetch,
  }),
}));

// connectAccount is the single connect entry (chooser mode, ADR078 §3b);
// install is the chooser's install-first action and nothing else.
const connectAccount = vi.fn();
const install = vi.fn();
vi.mock("@/features/git/hooks/use-connect-git", () => ({
  useConnectGitAccount: () => ({ connect: connectAccount, busy: false }),
  useConnectGit: () => ({ connect: install, busy: false }),
}));

const disconnect = vi.fn();
vi.mock("@/features/git/hooks/use-disconnect-git", () => ({
  useDisconnectGit: () => ({ disconnect, busy: false }),
}));

// The repo list belongs to the connection set: a bind or a disconnect here
// must not leave the picker on another page reading the old set's cache.
const forgetCachedRepos = vi.fn();
vi.mock("@/features/git/hooks/use-forget-cached-repos", () => ({
  useForgetCachedRepos: () => forgetCachedRepos,
}));

const select = vi.fn();
const selectionState = {
  candidates: [] as {
    installationId: number;
    accountLogin: string;
    connected: boolean;
  }[],
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
  gitState.loading = false;
  gitState.connections = [];
  refetch.mockReset();
  navigate.mockReset();
  toastSuccess.mockReset();
  connectAccount.mockReset();
  install.mockReset();
  disconnect.mockReset();
  disconnect.mockResolvedValue(true);
  forgetCachedRepos.mockReset();
  select.mockReset();
  select.mockResolvedValue(true);
  selectionState.candidates = [];
  selectionState.loading = false;
  selectionState.gone = false;
  selectionState.selecting = false;
});

const CONNECTED: Conn = {
  accountLogin: "bex-co",
  installationId: 2,
  createdAt: "2026-08-01",
  installUrl: "https://github.com/apps/bex-co/installations/new",
};

describe("ConnectGithubCard callback failures", () => {
  it("shows an expired callback as a visible error that restarts the single connect", async () => {
    const user = userEvent.setup();
    render(<ConnectGithubCard callbackError="expired_state" />);

    const alert = screen.getByRole("alert");
    expect(
      within(alert).getByText("GitHub connection wasn't completed"),
    ).toBeInTheDocument();
    expect(
      within(alert).getByText(
        "This connection request expired. Select Connect GitHub account to try again.",
      ),
    ).toBeInTheDocument();
    await user.click(
      within(alert).getByRole("button", { name: "Connect GitHub account" }),
    );
    expect(connectAccount).toHaveBeenCalledTimes(1);
    expect(install).not.toHaveBeenCalled();
  });

  it("does not reflect an unknown callback value", () => {
    render(<ConnectGithubCard callbackError="attacker-controlled-text" />);

    expect(
      screen.getByText(
        "GitHub couldn't complete the connection. Select Connect GitHub account to try again.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("attacker-controlled-text"),
    ).not.toBeInTheDocument();
  });

  // w1/m526 (DoD 5): an org member's install request waits on the owners; it
  // is said so plainly, and no restart is offered that could not include it.
  it("tells an install requester the request went to the org's owners", () => {
    render(<ConnectGithubCard callbackError="install_requested" />);

    const alert = screen.getByRole("alert");
    expect(
      within(alert).getByText(
        "Your request to install the bex GitHub App was sent to the organization's owners. Once one of them approves it, connect again.",
      ),
    ).toBeInTheDocument();
    expect(within(alert).queryByRole("button")).toBeNull();
  });

  // missing_state is the Configure dead end (a direct github.com install): its
  // recovery is the chooser, which lists the installed account.
  it("recovers a missing state through the single connect, not the install URL", async () => {
    const user = userEvent.setup();
    render(<ConnectGithubCard callbackError="missing_state" />);

    await user.click(
      within(screen.getByRole("alert")).getByRole("button", {
        name: "Connect GitHub account",
      }),
    );
    expect(connectAccount).toHaveBeenCalledTimes(1);
    expect(install).not.toHaveBeenCalled();
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

  it("explains a claim that found no administered account", () => {
    render(<ConnectGithubCard callbackError="no_claimable_installation" />);

    expect(
      screen.getByText(
        "No GitHub account you administer was found. Check that you authorized the right GitHub user, or install the bex GitHub App on the account first.",
      ),
    ).toBeInTheDocument();
  });
});

// w1/m526 (ADR078 §3b, DoD 1): one connect action, whatever the state, and it
// is the chooser-mode entry — never a second "claim" button to guess between.
describe("ConnectGithubCard single connect action", () => {
  it("offers exactly one action while disconnected", async () => {
    const user = userEvent.setup();
    render(<ConnectGithubCard />);

    expect(screen.getAllByRole("button")).toHaveLength(1);
    await user.click(
      screen.getByRole("button", { name: "Connect GitHub account" }),
    );
    expect(connectAccount).toHaveBeenCalledTimes(1);
    expect(install).not.toHaveBeenCalled();
    expect(document.body.textContent).not.toMatch(/claim/i);
  });

  it("offers exactly one connect action once connected", async () => {
    const user = userEvent.setup();
    gitState.connections = [CONNECTED];
    render(<ConnectGithubCard />);

    expect(
      screen.queryByRole("button", { name: "Connect GitHub account" }),
    ).toBeNull();
    await user.click(
      screen.getByRole("button", { name: "Connect another account" }),
    );
    expect(connectAccount).toHaveBeenCalledTimes(1);
    expect(install).not.toHaveBeenCalled();
    expect(document.body.textContent).not.toMatch(/claim/i);
  });
});

describe("ConnectGithubCard disconnect", () => {
  it("disconnects one account and forgets the cached repos", async () => {
    const user = userEvent.setup();
    gitState.connections = [CONNECTED];
    render(<ConnectGithubCard />);

    await user.click(screen.getByRole("button", { name: "Disconnect" }));
    await user.click(
      within(screen.getByRole("alertdialog")).getByRole("button", {
        name: "Disconnect",
      }),
    );

    await waitFor(() => expect(refetch).toHaveBeenCalled());
    expect(disconnect).toHaveBeenCalledWith(CONNECTED.installationId);
    expect(forgetCachedRepos).toHaveBeenCalledTimes(1);
  });
});

describe("ConnectGithubCard account chooser", () => {
  it("binds the chosen account, then closes, cleans the URL and says so", async () => {
    const user = userEvent.setup();
    selectionState.candidates = [
      { installationId: 1, accountLogin: "puncsky", connected: false },
      { installationId: 2, accountLogin: "bex-co", connected: true },
    ];

    render(<ConnectGithubCard claimSelectionId="gcs-1" />);

    expect(
      screen.getByText("Choose a GitHub account to connect"),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /puncsky/ }));

    expect(select).toHaveBeenCalledWith(1);
    await waitFor(() => expect(refetch).toHaveBeenCalled());
    expect(forgetCachedRepos).toHaveBeenCalledTimes(1);
    expect(toastSuccess).toHaveBeenCalledWith(
      "puncsky is connected to this workspace.",
    );
    expect(screen.queryByTestId("git-account-chooser")).toBeNull();
    // The selection id leaves the URL, replacing the history entry.
    expect(navigate).toHaveBeenCalledTimes(1);
    const nav = navigate.mock.calls[0][0];
    expect(nav).toMatchObject({
      to: "/settings",
      hash: "integrations",
      replace: true,
    });
    const next = nav.search({ git_claim_selection: "gcs-1", flow: "f-1" });
    expect(next.git_claim_selection).toBeUndefined();
    expect(next.flow).toBe("f-1");
  });

  it("marks an account this workspace already holds and does not offer it", () => {
    selectionState.candidates = [
      { installationId: 1, accountLogin: "puncsky", connected: false },
      { installationId: 2, accountLogin: "bex-co", connected: true },
    ];

    render(<ConnectGithubCard claimSelectionId="gcs-1" />);

    const chooser = screen.getByTestId("git-account-chooser");
    expect(within(chooser).getByText("bex-co")).toBeInTheDocument();
    expect(within(chooser).getByText("Connected")).toBeInTheDocument();
    expect(
      within(chooser).queryByRole("button", { name: /bex-co/ }),
    ).toBeNull();
    expect(
      within(chooser).getByRole("button", { name: /puncsky/ }),
    ).toBeEnabled();
  });

  it("installs on another account through the install-first flow only", async () => {
    const user = userEvent.setup();
    selectionState.candidates = [
      { installationId: 1, accountLogin: "puncsky", connected: false },
    ];

    render(<ConnectGithubCard claimSelectionId="gcs-1" />);

    await user.click(
      screen.getByRole("button", { name: "Install on another GitHub account" }),
    );
    expect(install).toHaveBeenCalledTimes(1);
    expect(connectAccount).not.toHaveBeenCalled();
    expect(select).not.toHaveBeenCalled();
  });

  it("offers only the install action when nothing is installed yet", () => {
    selectionState.candidates = [];

    render(<ConnectGithubCard claimSelectionId="gcs-empty" />);

    const chooser = screen.getByTestId("git-account-chooser");
    expect(
      within(chooser).getByText(
        "None of the GitHub accounts you administer has the bex GitHub App yet. Install it on one to connect it.",
      ),
    ).toBeInTheDocument();
    expect(
      within(chooser)
        .getAllByRole("button")
        .map((b) => b.textContent),
    ).toEqual(["Install on another GitHub account", "Cancel"]);
  });

  it("keeps the chooser when the choice did not bind", async () => {
    const user = userEvent.setup();
    select.mockResolvedValue(false);
    selectionState.candidates = [
      { installationId: 1, accountLogin: "puncsky", connected: false },
    ];

    render(<ConnectGithubCard claimSelectionId="gcs-1" />);
    await user.click(screen.getByRole("button", { name: /puncsky/ }));

    expect(screen.getByTestId("git-account-chooser")).toBeInTheDocument();
    expect(navigate).not.toHaveBeenCalled();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(forgetCachedRepos).not.toHaveBeenCalled();
  });

  it("closes on Cancel without binding", async () => {
    const user = userEvent.setup();
    selectionState.candidates = [
      { installationId: 1, accountLogin: "puncsky", connected: false },
    ];

    render(<ConnectGithubCard claimSelectionId="gcs-1" />);
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(screen.queryByTestId("git-account-chooser")).toBeNull();
    expect(select).not.toHaveBeenCalled();
    expect(navigate.mock.calls[0][0]).toMatchObject({ replace: true });
  });

  it("does not render the chooser without a selection id", () => {
    selectionState.candidates = [
      { installationId: 1, accountLogin: "octo-org", connected: false },
    ];

    render(<ConnectGithubCard />);

    expect(
      screen.queryByText("Choose a GitHub account to connect"),
    ).not.toBeInTheDocument();
  });

  it("offers a restart when the selection is expired or already spent", async () => {
    const user = userEvent.setup();
    selectionState.gone = true;

    render(<ConnectGithubCard claimSelectionId="gcs-stale" />);

    const alert = screen.getByRole("alert");
    expect(
      within(alert).getByText("This choice has expired"),
    ).toBeInTheDocument();
    expect(
      within(alert).getByText("Select Connect GitHub account to start again."),
    ).toBeInTheDocument();
    await user.click(
      within(alert).getByRole("button", { name: "Connect GitHub account" }),
    );
    expect(connectAccount).toHaveBeenCalledTimes(1);
  });

  it("previews the chooser's panel while its options load", () => {
    selectionState.loading = true;

    render(<ConnectGithubCard claimSelectionId="gcs-1" />);

    expect(
      screen.getByTestId("git-account-chooser-skeleton"),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("git-account-chooser")).toBeNull();
  });
});

// w4/686: the connections read's loading frame is the disconnected state's
// body and its one action at their ready geometry, its copy reserved.
describe("ConnectGithubCard loading frame", () => {
  it("reserves the body and the one action while loading (w4/686)", () => {
    const ready = render(<ConnectGithubCard />);
    const readyText = ready.container.querySelector(
      "[data-slot='card-content']",
    )!.textContent;
    expect(ready.container.querySelectorAll("button")).toHaveLength(1);
    ready.unmount();

    gitState.loading = true;
    const { container } = render(<ConnectGithubCard />);
    const frame = container.querySelector(
      "[data-testid='git-connections-skeleton']",
    )!;
    expect(frame).toHaveAttribute("aria-hidden", "true");
    // The same copy, laid out invisibly so it wraps as the ready card does.
    expect(frame.textContent).toBe(readyText);
    expect(frame.querySelectorAll(".invisible")).toHaveLength(2);
    // No action is offered before the read decides which state applies.
    expect(container.querySelector("button")).toBeNull();
  });

  // w4/686 reopened: the frame's copy sits in <p>s, and a <div> bar inside one
  // is moved out of it by the HTML parser, so every direct /settings load threw
  // React #418. The server markup must hydrate as it is.
  it("hydrates the loading frame's server markup without a mismatch (w4/686)", () => {
    gitState.loading = true;
    const now = Date.UTC(2026, 10, 16);
    const probe = hydrateAcrossBoundary(<ConnectGithubCard />, {
      serverNow: now,
      clientNow: now,
    });
    expect(probe.html).toContain("git-connections-skeleton");
    expect(probe.recovered).toEqual([]);
  });
});

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  AccessControlPanel,
  type AccessControlPanelProps,
} from "@/features/databases/components/access-control-panel";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { codedGraphQLError } from "@/test/mocks/apollo";

const createCredential = vi.fn();
const deleteCredential = vi.fn();
const resetPooled = vi.fn();
const mutate = vi.hoisted(() => vi.fn());
let credentials: { username: string; default: boolean }[] = [];
let pooled: { internal: string; external: string } | null = null;

// Only the connection-pool toggle writes through Apollo here; the access
// controls' own reads and writes come from the mocked useAccessControl.
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate, { loading: false }],
}));
vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({
    instanceTypes: [
      { id: "free", supportsConnectionPooling: false },
      { id: "basic-1gb", supportsConnectionPooling: true },
    ],
  }),
}));
vi.mock("@/features/databases/hooks/use-access-control", () => ({
  useAccessControl: () => ({
    allowList: [],
    credentials,
    loading: false,
    savingAllowList: false,
    creatingCredential: false,
    saveAllowList: vi.fn(),
    createCredential,
    deleteCredential,
    pooled,
    poolLoading: false,
    revealPooled: vi.fn(),
    resetPooled,
  }),
}));

beforeEach(() => {
  createCredential.mockReset();
  deleteCredential.mockReset();
  resetPooled.mockReset();
  mutate.mockReset().mockResolvedValue({ data: {} });
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  credentials = [];
  pooled = null;
});

type PanelOverrides = Partial<AccessControlPanelProps["database"]> & {
  onChanged?: () => void;
};

function panel({ onChanged = vi.fn(), ...database }: PanelOverrides = {}) {
  return (
    <AccessControlPanel
      database={{
        id: "dpg-source",
        name: "shop",
        plan: "free",
        poolerEnabled: false,
        highAvailabilityEnabled: false,
        ...database,
      }}
      onChanged={onChanged}
    />
  );
}

function renderPanel(overrides: PanelOverrides = {}) {
  return render(panel(overrides));
}

// w5/m265: Render's Credentials section — a new credential becomes the
// default, the default cannot be deleted, and no password is shown here.
describe("AccessControlPanel credentials", () => {
  it("marks the default and refuses to delete it, with the reason", async () => {
    credentials = [
      { username: "dpg_x_user", default: false },
      { username: "rotated", default: true },
    ];
    deleteCredential.mockResolvedValue(undefined);
    const user = userEvent.setup();
    renderPanel();

    const locked = screen.getByRole("button", { name: "Delete rotated" });
    expect(locked).toBeDisabled();
    expect(locked).toHaveAttribute(
      "title",
      "The default credential can't be deleted. Add a new default credential first.",
    );
    expect(screen.getAllByText("Default")).toHaveLength(1);

    await user.click(screen.getByRole("button", { name: "Delete dpg_x_user" }));
    expect(deleteCredential).toHaveBeenCalledWith("dpg_x_user");
  });

  it("creates a new default credential and clears the input, showing no password", async () => {
    createCredential.mockResolvedValue(true);
    const user = userEvent.setup();
    renderPanel();

    const name = screen.getByRole("textbox", {
      name: "New credential username",
    });
    await user.type(name, "rotated");
    await user.click(
      screen.getByRole("button", { name: "New default credential" }),
    );

    expect(createCredential).toHaveBeenCalledWith("rotated");
    expect(name).toHaveValue("");
    // Render echoes no password; it is read through the connection info.
    expect(
      screen.queryByRole("button", { name: /copy/i }),
    ).not.toBeInTheDocument();
  });

  it("keeps the username recoverable on failure", async () => {
    createCredential.mockResolvedValue(false);
    const user = userEvent.setup();
    renderPanel();

    const name = screen.getByRole("textbox", {
      name: "New credential username",
    });
    await user.type(name, "analytics");
    await user.click(
      screen.getByRole("button", { name: "New default credential" }),
    );
    expect(name).toHaveValue("analytics");
  });

  // w4/m170: adding "postgres" wedged the database; say so before the click.
  it("refuses a reserved role name before submit", async () => {
    const user = userEvent.setup();
    renderPanel();
    const name = screen.getByRole("textbox", {
      name: "New credential username",
    });
    await user.type(name, "postgres");
    const add = screen.getByRole("button", { name: "New default credential" });
    expect(add).toBeDisabled();
    expect(
      screen.getByText(
        "“postgres” is reserved by PostgreSQL. Choose another name.",
      ),
    ).toBeInTheDocument();
    expect(createCredential).not.toHaveBeenCalled();

    await user.clear(name);
    await user.type(name, "qa_extra");
    expect(add).toBeEnabled();
  });

  it("names the allowlist inputs", () => {
    renderPanel();
    expect(
      screen.getByRole("textbox", { name: "New CIDR block" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: "New rule description" }),
    ).toBeInTheDocument();
  });
});

describe("AccessControlPanel connection pooling", () => {
  it("explains the plan restriction instead of offering a refused enable call", () => {
    pooled = { internal: "", external: "" };
    renderPanel();
    expect(
      screen.getByText(/This plan does not support connection pooling/),
    ).toBeVisible();
    expect(screen.queryByText(/PATCH \/v1\/postgres/)).not.toBeInTheDocument();
  });

  // w2/m307: the copy points at the switch, not at the REST call (w4/107).
  it("points a supported plan without a pooler at the switch", () => {
    renderPanel({ plan: "basic-1gb" });
    expect(
      screen.getByText(
        "Turn on connection pooling above to get pooled connection strings.",
      ),
    ).toBeVisible();
    expect(screen.queryByText(/PATCH \/v1\/postgres/)).not.toBeInTheDocument();
    // Nothing to reveal while pooling is off.
    expect(
      screen.queryByRole("button", { name: "Reveal pooled strings" }),
    ).not.toBeInTheDocument();
  });

  it("offers the reveal once pooling is on, and says when the pooler is still starting", () => {
    const view = renderPanel({ plan: "basic-1gb", poolerEnabled: true });
    expect(
      screen.getByRole("button", { name: "Reveal pooled strings" }),
    ).toBeEnabled();
    pooled = { internal: "", external: "" };
    view.rerender(panel({ plan: "basic-1gb", poolerEnabled: true }));
    expect(
      screen.getByText(/Connection pooling is starting/),
    ).toBeInTheDocument();
    // Reveal stays, so the strings can be read once the pooler is up.
    expect(
      screen.getByRole("button", { name: "Reveal pooled strings" }),
    ).toBeEnabled();
  });

  it("keeps existing pooled connections visible on an unsupported plan", () => {
    pooled = { internal: "postgresql://existing-pool", external: "" };
    renderPanel({ poolerEnabled: true });
    expect(screen.getByText("postgresql://existing-pool")).toBeVisible();
    expect(
      screen.queryByText(/This plan does not support connection pooling/),
    ).not.toBeInTheDocument();
    // A pooler left on a free plan can still be turned off, as bex-api allows.
    expect(
      screen.getByRole("switch", { name: "Connection pool" }),
    ).toBeEnabled();
  });
});

// w2/m307: Render's Info > Connection Pool toggle. Either direction asks
// first, because it restarts the database — or fails over to the standby
// when HA is on — and the confirm button says what it does.
describe("AccessControlPanel connection pool switch (w2/m307)", () => {
  const poolSwitch = () =>
    screen.getByRole("switch", { name: "Connection pool" });

  it("enables PgBouncer after a confirmation that names the restart", async () => {
    const onChanged = vi.fn();
    const user = userEvent.setup();
    renderPanel({ plan: "basic-1gb", onChanged });
    expect(poolSwitch()).not.toBeChecked();
    await user.click(poolSwitch());
    expect(mutate).not.toHaveBeenCalled();

    const dialog = screen.getByRole("alertdialog");
    expect(
      within(dialog).getByText("Enable connection pooling for shop?"),
    ).toBeInTheDocument();
    expect(dialog).toHaveTextContent(
      "Enabling connection pooling restarts the database, which causes a few minutes of downtime.",
    );
    expect(dialog).not.toHaveTextContent(/fails over/);

    await user.click(
      within(dialog).getByRole("button", { name: "Enable connection pool" }),
    );
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "dpg-source", connectionPool: "pgbouncer" },
    });
    expect(onChanged).toHaveBeenCalledOnce();
    // The revealed snapshot belongs to the old pooler state.
    expect(resetPooled).toHaveBeenCalledOnce();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("says a database with HA fails over to the standby instead", async () => {
    const user = userEvent.setup();
    renderPanel({ plan: "basic-1gb", highAvailabilityEnabled: true });
    await user.click(poolSwitch());
    const dialog = screen.getByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "Enabling connection pooling fails over to the standby instead of restarting the database",
    );
    expect(dialog).not.toHaveTextContent(/few minutes of downtime/);
  });

  it("disables the pooler with its own confirmation", async () => {
    const onChanged = vi.fn();
    const user = userEvent.setup();
    renderPanel({ plan: "basic-1gb", poolerEnabled: true, onChanged });
    expect(poolSwitch()).toBeChecked();
    await user.click(poolSwitch());

    const dialog = screen.getByRole("alertdialog");
    expect(
      within(dialog).getByText("Disable connection pooling for shop?"),
    ).toBeInTheDocument();
    expect(dialog).toHaveTextContent(
      "The pooled connection strings stop working.",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Disable connection pool" }),
    );
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "dpg-source", connectionPool: "none" },
    });
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("names the failover when disabling with HA on", async () => {
    const user = userEvent.setup();
    renderPanel({
      plan: "basic-1gb",
      poolerEnabled: true,
      highAvailabilityEnabled: true,
    });
    await user.click(poolSwitch());
    expect(screen.getByRole("alertdialog")).toHaveTextContent(
      "Disabling connection pooling fails over to the standby instead of restarting the database",
    );
  });

  it("keeps the switch off and disabled on a free plan, and says why", () => {
    renderPanel({ plan: "free" });
    expect(poolSwitch()).toBeDisabled();
    expect(poolSwitch()).not.toBeChecked();
    expect(
      screen.getByText(/This plan does not support connection pooling/),
    ).toBeVisible();
  });

  it("disables the switch for a member who cannot operate the database", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        canOperate: false,
        reasonKey: (action) =>
          action === "can_operate" ? "databases.haFailover" : undefined,
      }),
    );
    renderPanel({ plan: "basic-1gb" });
    expect(poolSwitch()).toBeDisabled();
  });

  it("keeps the confirmation open and does not refetch when the API refuses", async () => {
    mutate.mockRejectedValueOnce(
      codedGraphQLError("POSTGRES_CONNECTION_POOL_PLAN_UNSUPPORTED"),
    );
    const onChanged = vi.fn();
    const user = userEvent.setup();
    renderPanel({ plan: "basic-1gb", onChanged });
    await user.click(poolSwitch());
    await user.click(
      screen.getByRole("button", { name: "Enable connection pool" }),
    );
    expect(mutate).toHaveBeenCalledOnce();
    expect(onChanged).not.toHaveBeenCalled();
    expect(resetPooled).not.toHaveBeenCalled();
    expect(
      screen.getByRole("button", { name: "Enable connection pool" }),
    ).toBeInTheDocument();
  });
});

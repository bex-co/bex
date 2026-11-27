import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { memoryStorage } from "@/test/mocks/storage";
import { DatabaseVersionUpgrade } from "@/features/databases/components/database-version-upgrade";
import type {
  DatabaseDetailView,
  DatabaseUpgradeRunView,
} from "@/features/databases/types";

// w3/m676: Render's guided version-upgrade page, section by section.

const recover = vi.hoisted(() => vi.fn());
const recoveryInfo = vi.hoisted(() => ({
  current: {
    enabled: true,
    earliestRecoveryTime: "2026-11-25T00:00:00Z",
    latestRecoveryTime: "2026-11-26T09:00:00Z",
  } as Record<string, unknown> | null,
}));
vi.mock("@apollo/client/react", () => ({
  useQuery: () => ({
    data: recoveryInfo.current
      ? { databaseRecoveryInfo: recoveryInfo.current }
      : undefined,
    loading: false,
    startPolling: vi.fn(),
    stopPolling: vi.fn(),
  }),
  useMutation: () => [recover, { loading: false }],
}));

const extensions = vi.hoisted(() => ({
  current: [] as { name: string; blocksUpgradeDatabase: boolean }[],
}));
vi.mock("@/features/databases/hooks/use-database-extensions", () => ({
  useDatabaseExtensions: () => ({
    extensions: extensions.current,
    loading: false,
  }),
}));

const updateVersion = vi.hoisted(() => vi.fn());
const upgradeError = vi.hoisted(() => ({ current: null as string | null }));
vi.mock("@/features/databases/hooks/use-update-database-version", () => ({
  useUpdateDatabaseVersion: () => ({
    updateVersion,
    busy: false,
    error: upgradeError.current,
    clearError: vi.fn(),
  }),
}));

const acknowledge = vi.hoisted(() => vi.fn());
vi.mock("@/features/databases/hooks/use-acknowledge-database-upgrade", () => ({
  useAcknowledgeDatabaseUpgrade: () => ({ acknowledge, busy: false }),
}));

vi.mock("@/features/logs/components/datastore-log-viewer", () => ({
  DatastoreLogViewer: (props: {
    resource: string;
    range: { id: string; startTime?: string; endTime?: string };
    emptyBody?: string;
  }) => (
    <div
      data-testid="upgrade-logs"
      data-resource={props.resource}
      data-range={props.range.id}
      data-start={props.range.startTime}
      data-end={props.range.endTime}
    >
      {props.emptyBody}
    </div>
  ),
}));

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    params,
    to,
    hash,
    className,
  }: {
    children: ReactNode;
    params: { databaseId: string };
    to: string;
    hash?: string;
    className?: string;
  }) => (
    <a
      href={`${to.replace("$databaseId", params.databaseId)}${hash ? `#${hash}` : ""}`}
      className={className}
    >
      {children}
    </a>
  ),
}));

const toastSuccess = vi.hoisted(() => vi.fn());
const toastError = vi.hoisted(() => vi.fn());
vi.mock("sonner", () => ({
  toast: { success: toastSuccess, error: toastError, warning: vi.fn() },
}));

const DATABASE: DatabaseDetailView = {
  id: "dpg-orders",
  name: "orders",
  status: "available",
  plan: "basic-1gb",
  version: "16",
  diskSizeGB: 15,
  createdAt: null,
  public: false,
  suspended: "not_suspended",
  databaseName: "orders",
  databaseUser: "orders_user",
  highAvailabilityEnabled: false,
  diskAutoscalingEnabled: false,
  poolerEnabled: false,
  readReplicas: [],
  externalHost: null,
  backupsEnabled: true,
  lastUpgradeRun: null,
  versionUpgradeAvailable: "18",
};

function run(state: string, acknowledged = false): DatabaseUpgradeRunView {
  return {
    runId: "pgu-run1",
    startedAt: "2026-11-26T10:00:00Z",
    state,
    toVersion: "18",
    acknowledged,
  };
}

function page(overrides: Partial<DatabaseDetailView> = {}) {
  return render(
    <DatabaseVersionUpgrade database={{ ...DATABASE, ...overrides }} />,
  );
}

beforeEach(() => {
  recover.mockReset();
  updateVersion.mockReset();
  acknowledge.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  extensions.current = [];
  upgradeError.current = null;
  recoveryInfo.current = {
    enabled: true,
    earliestRecoveryTime: "2026-11-25T00:00:00Z",
    latestRecoveryTime: "2026-11-26T09:00:00Z",
  };
  // The clone pairing is kept in localStorage, as Render keeps it.
  vi.stubGlobal("localStorage", memoryStorage());
});

describe("the guided version-upgrade page", () => {
  it("says so, with no steps, on the latest major", () => {
    page({ version: "18" });
    expect(screen.getByText("Current version: PostgreSQL 18")).toBeVisible();
    expect(
      screen.getByText(
        "This database is running the latest supported version of PostgreSQL.",
      ),
    ).toBeVisible();
    expect(screen.queryByText("Steps to upgrade")).toBeNull();
    expect(screen.queryByRole("button", { name: /Upgrade to/ })).toBeNull();
  });

  it("offers the clone first, then this database, at the latest major only", () => {
    page();
    const steps = within(screen.getByRole("list", { name: "Upgrade steps" }));
    const items = steps.getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent(
      "Upgrade a clone of this database (recommended)",
    );
    expect(items[1]).toHaveTextContent("Upgrade this database");
    expect(
      screen.getByRole("button", { name: "Upgrade to PostgreSQL 18" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("link", { name: "Read release notes" }),
    ).toHaveAttribute(
      "href",
      "https://www.postgresql.org/docs/18/release-18.html",
    );
    // What the upgrade changes for good: PITR restarts, and no way back.
    expect(
      screen.getByText(/Point-in-Time Recovery starts again from the upgrade/),
    ).toBeVisible();
    expect(
      screen.getByText("The database can't return to PostgreSQL 16."),
    ).toBeVisible();
  });

  it("has no clone way without point-in-time recovery, nor on a clone", () => {
    const { unmount } = page({ backupsEnabled: false });
    expect(
      screen.queryByText("Upgrade a clone of this database (recommended)"),
    ).toBeNull();
    expect(
      screen.queryByText(/Point-in-Time Recovery starts again/),
    ).toBeNull();
    unmount();
    page({ name: "orders-upgrade-clone-ab12" });
    expect(
      screen.queryByText("Upgrade a clone of this database (recommended)"),
    ).toBeNull();
    expect(screen.getByText("Upgrade this database")).toBeVisible();
  });

  it("holds the clone and the upgrade until PITR is ready, a clone excepted", () => {
    recoveryInfo.current = {
      enabled: true,
      earliestRecoveryTime: null,
      latestRecoveryTime: null,
    };
    const { unmount } = page();
    expect(
      screen.getByRole("button", { name: "Clone this database" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Upgrade to PostgreSQL 18" }),
    ).toBeDisabled();
    unmount();
    page({ name: "orders-upgrade-clone-ab12" });
    expect(
      screen.getByRole("button", { name: "Upgrade to PostgreSQL 18" }),
    ).toBeEnabled();
  });

  it("names the extensions that block the upgrade and holds both ways", () => {
    extensions.current = [
      { name: "adminpack", blocksUpgradeDatabase: true },
      { name: "vector", blocksUpgradeDatabase: false },
    ];
    page();
    const notice = screen.getByText(
      "Extensions block upgrading",
    ).parentElement!;
    expect(within(notice).getByText("adminpack")).toBeVisible();
    expect(within(notice).queryByText("vector")).toBeNull();
    expect(
      screen.getByRole("button", { name: "Clone this database" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Upgrade to PostgreSQL 18" }),
    ).toBeDisabled();
  });

  it("starts the upgrade at the target, and shows a guard's refusal inline", async () => {
    upgradeError.current =
      "a completed physical backup is required before upgrading this durable Postgres instance";
    page();
    await userEvent.click(
      screen.getByRole("button", { name: "Upgrade to PostgreSQL 18" }),
    );
    expect(updateVersion).toHaveBeenCalledWith("dpg-orders", "18");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "a completed physical backup is required",
    );
  });

  it("clones from the page and links the clone, remembered both ways", async () => {
    recover.mockResolvedValue({
      data: { recoverDatabase: { id: "dpg-clone" } },
    });
    page();
    await userEvent.click(
      screen.getByRole("button", { name: "Clone this database" }),
    );
    const name = recover.mock.calls[0][0].variables.name as string;
    expect(recover).toHaveBeenCalledWith({
      variables: { id: "dpg-orders", name },
    });
    expect(name).toMatch(/^orders-upgrade-clone-[a-z0-9]{4}$/);
    expect(
      await screen.findByRole("link", { name: "View Postgres clone" }),
    ).toHaveAttribute("href", "/databases/dpg-clone/version-upgrade");
    const stored = JSON.parse(
      window.localStorage.getItem("postgres-upgrade-clone-info")!,
    );
    expect(stored["dpg-clone"]).toEqual({ id: "dpg-orders", name: "orders" });
    expect(stored["dpg-orders"]).toEqual({ id: "dpg-clone", name });
  });

  it("shows the running upgrade's status and live logs", () => {
    page({ status: "upgrading", lastUpgradeRun: run("IN_PROGRESS") });
    const status = screen.getByRole("region", {
      name: "Upgrade status and logs",
    });
    expect(within(status).getByText("in progress")).toBeVisible();
    expect(within(status).getByTestId("upgrade-logs")).toHaveAttribute(
      "data-range",
      "live",
    );
    expect(screen.getByRole("button", { name: "Upgrading..." })).toBeDisabled();
    expect(screen.queryByText("Last Upgrade Attempt")).toBeNull();
  });

  it("keeps a succeeded upgrade as the last attempt, with its logs window", () => {
    page({ version: "18", lastUpgradeRun: run("SUCCEEDED") });
    expect(
      screen.getByText(
        "This database is running the latest supported version of PostgreSQL.",
      ),
    ).toBeVisible();
    expect(screen.getByText("Last Upgrade Attempt")).toBeVisible();
    expect(screen.getByText("succeeded")).toBeVisible();
    const logs = screen.getByTestId("upgrade-logs");
    expect(logs).toHaveAttribute("data-range", "custom");
    expect(logs).toHaveAttribute("data-start", "2026-11-26T10:00:00.000Z");
    expect(logs).toHaveAttribute("data-end", "2026-11-26T12:00:00.000Z");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows a failed upgrade's banner until it is dismissed, and the attempt after", async () => {
    const { unmount } = page({ lastUpgradeRun: run("FAILED") });
    const banner = screen.getByRole("alert");
    expect(banner).toHaveTextContent("Upgrade failed.");
    expect(banner).toHaveTextContent(
      "Your database is still running on its current version.",
    );
    await userEvent.click(
      within(banner).getByRole("button", { name: "Dismiss error" }),
    );
    expect(acknowledge).toHaveBeenCalledWith("dpg-orders", "pgu-run1");
    expect(screen.getByText("failed")).toBeVisible();
    // A failed upgrade leaves the database on its major: the steps stay.
    expect(screen.getByText("Steps to upgrade")).toBeVisible();
    unmount();
    page({ lastUpgradeRun: run("FAILED", true) });
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByText("Last Upgrade Attempt")).toBeVisible();
  });

  it("toasts the result of an upgrade that ends while the page is open", () => {
    const { rerender } = page({
      status: "upgrading",
      lastUpgradeRun: run("IN_PROGRESS"),
    });
    expect(toastSuccess).not.toHaveBeenCalled();
    rerender(
      <DatabaseVersionUpgrade
        database={{
          ...DATABASE,
          version: "18",
          lastUpgradeRun: run("SUCCEEDED"),
        }}
      />,
    );
    expect(toastSuccess).toHaveBeenCalledWith("Upgrade succeeded", {
      description: "Your database was updated to Postgres 18.",
    });
  });

  it("toasts nothing for a run already over when the page opened", () => {
    page({ lastUpgradeRun: run("FAILED") });
    expect(toastError).not.toHaveBeenCalled();
    expect(toastSuccess).not.toHaveBeenCalled();
  });
});

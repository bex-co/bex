import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { memoryStorage } from "@/test/mocks/storage";
import {
  DatabaseUpgradeAvailableBanner,
  DatabaseUpgradeCloneBanner,
  DatabaseVersionValue,
} from "@/features/databases/components/database-upgrade-notices";
import { rememberUpgradeClone } from "@/features/databases/lib/version-upgrade";
import type { DatabaseDetailView } from "@/features/databases/types";

// w3/m676: what a database's other pages say about its version upgrade.

vi.mock("@apollo/client/react", () => ({
  useMutation: () => [vi.fn(), { loading: false }],
}));

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    params,
    to,
    hash,
  }: {
    children: ReactNode;
    params: { databaseId: string };
    to: string;
    hash?: string;
  }) => (
    <a
      href={`${to.replace("$databaseId", params.databaseId)}${hash ? `#${hash}` : ""}`}
    >
      {children}
    </a>
  ),
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

beforeEach(() => {
  vi.stubGlobal("localStorage", memoryStorage());
});

describe("Info's version value", () => {
  const running: Partial<DatabaseDetailView> = {
    status: "upgrading",
    lastUpgradeRun: {
      runId: "pgu-1",
      startedAt: "2026-11-26T10:00:00Z",
      state: "IN_PROGRESS",
      toVersion: "18",
      acknowledged: false,
    },
  };
  it.each<[string, Partial<DatabaseDetailView>, string, string]>([
    ["an older major", {}, "PostgreSQL 16", "Upgrade version"],
    ["a running upgrade", running, "PostgreSQL 16", "View upgrade"],
    [
      "the latest major",
      { version: "18" },
      "PostgreSQL 18",
      "Review version details",
    ],
  ])("links the guided page on %s", (_, overrides, version, label) => {
    render(<DatabaseVersionValue database={{ ...DATABASE, ...overrides }} />);
    expect(screen.getByText(version)).toBeVisible();
    expect(screen.getByRole("link", { name: label })).toHaveAttribute(
      "href",
      "/databases/dpg-orders/version-upgrade",
    );
  });

  it("is the version alone on a free database, which has no upgrade page", () => {
    render(<DatabaseVersionValue database={{ ...DATABASE, plan: "free" }} />);
    expect(screen.getByText("PostgreSQL 16")).toBeVisible();
    expect(screen.queryByRole("link")).toBeNull();
  });
});

describe("Info's newer-major banner", () => {
  it("announces the latest major until it is dismissed in this browser", async () => {
    const { rerender } = render(
      <DatabaseUpgradeAvailableBanner database={DATABASE} />,
    );
    expect(screen.getByText("PostgreSQL 18 is available.")).toBeVisible();
    expect(
      screen.getByRole("link", { name: "View upgrade options" }),
    ).toHaveAttribute("href", "/databases/dpg-orders/version-upgrade");
    await userEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(screen.queryByText("PostgreSQL 18 is available.")).toBeNull();
    rerender(<DatabaseUpgradeAvailableBanner database={{ ...DATABASE }} />);
    expect(screen.queryByText("PostgreSQL 18 is available.")).toBeNull();
  });

  it("is absent on the latest major and on a free database", () => {
    const { rerender } = render(
      <DatabaseUpgradeAvailableBanner
        database={{ ...DATABASE, version: "18" }}
      />,
    );
    expect(screen.queryByRole("status")).toBeNull();
    rerender(
      <DatabaseUpgradeAvailableBanner
        database={{ ...DATABASE, plan: "free" }}
      />,
    );
    expect(screen.queryByRole("status")).toBeNull();
  });
});

describe("a test-upgrade clone's banner", () => {
  const CLONE: DatabaseDetailView = {
    ...DATABASE,
    id: "dpg-clone",
    name: "orders-upgrade-clone-ab12",
  };

  it("names the source the clone was made from in this browser", () => {
    act(() =>
      rememberUpgradeClone(
        { id: "dpg-clone", name: CLONE.name },
        { id: "dpg-orders", name: "orders" },
      ),
    );
    render(<DatabaseUpgradeCloneBanner database={CLONE} />);
    expect(
      screen.getByText("This database is a clone of orders."),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Delete this clone" }),
    ).toHaveAttribute("href", "/databases/dpg-clone#danger-zone");
    expect(
      screen.getByRole("link", { name: "Go to original database" }),
    ).toHaveAttribute("href", "/databases/dpg-orders/version-upgrade");
  });

  it("is absent without the pairing, and on the source itself", () => {
    const { rerender } = render(
      <DatabaseUpgradeCloneBanner database={CLONE} />,
    );
    expect(screen.queryByRole("status")).toBeNull();
    act(() =>
      rememberUpgradeClone(
        { id: "dpg-clone", name: CLONE.name },
        { id: "dpg-orders", name: "orders" },
      ),
    );
    rerender(<DatabaseUpgradeCloneBanner database={DATABASE} />);
    expect(screen.queryByRole("status")).toBeNull();
  });
});

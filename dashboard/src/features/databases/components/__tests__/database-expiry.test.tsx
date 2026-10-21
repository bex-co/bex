import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { DatabaseDetailView } from "@/features/databases/types";

vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({
    instanceTypes: [],
    loading: false,
    error: undefined,
  }),
}));
vi.mock("@/features/databases/components/database-name-row", () => ({
  DatabaseNameRow: ({ database }: { database: { name: string } }) => (
    <div>{database.name}</div>
  ),
}));
vi.mock("@/features/databases/components/database-version-control", () => ({
  DatabaseVersionControl: () => <div data-testid="version-control" />,
}));

import { DatabaseMetadataCard } from "@/features/databases/components/database-metadata-card";
import { DatabaseExpiryHint } from "@/features/databases/components/database-expiry";

const EXPIRES = "2026-12-02T09:30:00Z";
const DAY = 24 * 60 * 60 * 1000;

function db(overrides: Partial<DatabaseDetailView> = {}): DatabaseDetailView {
  return {
    id: "dpg-orders",
    name: "orders",
    status: "available",
    plan: "free",
    version: "16",
    diskSizeGB: 1,
    createdAt: null,
    expiresAt: EXPIRES,
    public: false,
    suspended: "not_suspended",
    databaseName: "orders",
    databaseUser: "orders_user",
    highAvailabilityEnabled: false,
    diskAutoscalingEnabled: false,
    poolerEnabled: false,
    readReplicas: [],
    externalHost: null,
    backupsEnabled: false,
    ...overrides,
  };
}

function renderCard(database: DatabaseDetailView) {
  render(
    <DatabaseMetadataCard
      database={database}
      onVersionChanged={vi.fn()}
      onRenamed={vi.fn()}
    />,
  );
}

// w8/m126: a free database shows when it expires; near or past expiry it warns
// and links to the Plan section's upgrade; a paid one has no Expires row.
describe("free database expiry on the database page", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows the expiry of a free database without a warning far from it", () => {
    vi.setSystemTime(Date.parse(EXPIRES) - 20 * DAY);
    renderCard(db());
    expect(screen.getByText("Expires")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("omits the row for a paid database", () => {
    vi.setSystemTime(Date.parse(EXPIRES) - 2 * DAY);
    renderCard(db({ plan: "basic-1gb", expiresAt: null }));
    expect(screen.queryByText("Expires")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("warns within seven days and links to the upgrade", () => {
    vi.setSystemTime(Date.parse(EXPIRES) - 2 * DAY);
    renderCard(db());
    expect(screen.getByRole("alert")).toHaveTextContent(
      "This free database expires on 2026-12-02 UTC",
    );
    expect(
      screen.getByRole("link", { name: "Upgrade to a paid plan" }),
    ).toHaveAttribute("href", "#plan");
  });

  it("names the deletion date and the expiry reason once expired", () => {
    vi.setSystemTime(Date.parse(EXPIRES) + DAY);
    renderCard(
      db({
        status: "suspended",
        suspended: "suspended",
        statusReasonCode: "POSTGRES_FREE_EXPIRED",
        statusReason: "This free database expired on …",
      }),
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "deleted on 2026-12-16 UTC",
    );
    expect(
      screen.getByText(
        "This free database has expired. Upgrade it to a paid plan to restore it.",
      ),
    ).toBeInTheDocument();
  });

  it("hints in the list only near or past expiry", () => {
    vi.setSystemTime(Date.parse(EXPIRES) - 20 * DAY);
    const { rerender, container } = render(
      <DatabaseExpiryHint expiresAt={EXPIRES} />,
    );
    expect(container).toBeEmptyDOMElement();
    rerender(<DatabaseExpiryHint expiresAt={null} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("hints Expires soon and Expired", () => {
    vi.setSystemTime(Date.parse(EXPIRES) - DAY);
    const soon = render(<DatabaseExpiryHint expiresAt={EXPIRES} />);
    expect(soon.getByText("Expires soon")).toBeInTheDocument();
    soon.unmount();
    vi.setSystemTime(Date.parse(EXPIRES) + DAY);
    render(<DatabaseExpiryHint expiresAt={EXPIRES} />);
    expect(screen.getByText("Expired")).toBeInTheDocument();
  });
});

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { UsagePage } from "../usage-page";
import { useUsage } from "@/features/usage/hooks/use-usage";
import { useUsageTrend } from "@/features/usage/hooks/use-usage-trend";

vi.mock("@/features/usage/hooks/use-usage", () => ({
  useUsage: vi.fn(),
}));

vi.mock("@/features/usage/hooks/use-usage-trend", () => ({
  useUsageTrend: vi.fn(),
}));

vi.mock("@/features/usage/components/resource-caps", () => ({
  WorkspaceResourceCaps: () => <div data-testid="resource-caps" />,
}));

vi.mock("@/features/usage/components/billing-onboarding", () => ({
  BillingOnboardingCard: () => <div data-testid="billing-onboarding" />,
}));

vi.mock("@/features/usage/lib/period", () => ({
  periodFor: vi.fn((n: number) => {
    const d = new Date(2026, 6, 15); // fixed: July 2026
    d.setMonth(d.getMonth() - n);
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
  }),
  periodLabel: vi.fn((period: string) => period),
}));

// DashboardLayout brings in the router and sidebar — stub it to a simple wrapper.
vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="dashboard-layout">{children}</div>
  ),
}));

// SvgLineChart brings in metrics internals — stub to a lightweight placeholder.
vi.mock("@/features/metrics/components/svg-line-chart", () => ({
  SvgLineChart: () => <div data-testid="svg-line-chart" />,
}));

const mockUseUsage = vi.mocked(useUsage);
const mockUseUsageTrend = vi.mocked(useUsageTrend);

function emptyTrendState() {
  return {
    points: [
      {
        period: "2026-05",
        timestamp: new Date(2026, 4, 1).toISOString(),
        compute: 0,
        bandwidth: 0,
        build: 0,
        storage: 0,
      },
      {
        period: "2026-06",
        timestamp: new Date(2026, 5, 1).toISOString(),
        compute: 0,
        bandwidth: 0,
        build: 0,
        storage: 0,
      },
      {
        period: "2026-07",
        timestamp: new Date(2026, 6, 1).toISOString(),
        compute: 0,
        bandwidth: 0,
        build: 0,
        storage: 0,
      },
    ],
    loading: false,
  };
}

function loadingState() {
  return { summary: null, loading: true, error: undefined };
}

function emptyState() {
  return {
    summary: { workspaceId: "ws", period: "2026-07", services: [] },
    loading: false,
    error: undefined,
  };
}

function dataState() {
  return {
    summary: {
      workspaceId: "ws-abc",
      period: "2026-07",
      services: [
        {
          serviceId: "srv-cms",
          serviceName: "eden-cms-v2",
          resourceKind: "service",
          rows: [
            { kind: "instance_seconds", tier: "starter", total: 7200 },
            { kind: "egress_bytes", tier: "", total: 524288000 },
            { kind: "build_seconds", tier: "", total: 1800 },
          ],
        },
        {
          serviceId: "email-worker",
          serviceName: "",
          resourceKind: "service",
          rows: [{ kind: "instance_seconds", tier: "hobby", total: 3600 }],
        },
        {
          serviceId: "dbs-shared",
          serviceName: "shared",
          resourceKind: "postgres",
          rows: [
            { kind: "instance_seconds", tier: "free", total: 3600 },
            { kind: "storage_gb_seconds", tier: "", total: 7200 },
          ],
        },
        {
          serviceId: "red-shared",
          serviceName: "shared",
          resourceKind: "key_value",
          rows: [
            { kind: "instance_seconds", tier: "free", total: 1800 },
            { kind: "storage_gb_seconds", tier: "", total: 3600 },
          ],
        },
      ],
    },
    loading: false,
    error: undefined,
  };
}

describe("UsagePage", () => {
  beforeEach(() => {
    mockUseUsage.mockReset();
    mockUseUsageTrend.mockReset();
    mockUseUsageTrend.mockReturnValue(emptyTrendState());
  });

  it("renders the page heading and subtitle", () => {
    mockUseUsage.mockReturnValue(emptyState());

    render(<UsagePage />);

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "Billing",
    );
    expect(
      screen.getByText(
        "Payment, invoices, and month-to-date workspace consumption",
      ),
    ).toBeInTheDocument();
    // The two-section order (w5/m70): Billing heading precedes Usage heading.
    const sections = screen.getAllByRole("heading", { level: 2 });
    expect(sections[0]).toHaveTextContent("Billing");
    expect(
      screen.getByRole("heading", { level: 2, name: "Usage" }),
    ).toBeInTheDocument();
  });

  it("renders all quantity section cards", () => {
    mockUseUsage.mockReturnValue(emptyState());

    render(<UsagePage />);

    // "Compute" and "Bandwidth" appear both as card titles and as trend section
    // meter labels — use getAllByText and verify at least one match each.
    expect(screen.getAllByText("Compute").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Bandwidth").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Build Minutes").length).toBeGreaterThanOrEqual(
      1,
    );
    expect(screen.getAllByText("Storage").length).toBeGreaterThanOrEqual(1);
  });

  it("shows skeleton rows while loading and no summary is available", () => {
    mockUseUsage.mockReturnValue(loadingState());

    const { container } = render(<UsagePage />);

    // Skeleton elements are rendered as animated divs
    const skeletons = container.querySelectorAll(
      "[class*='animate-pulse'], [data-slot='skeleton']",
    );
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("shows 'No usage recorded this month.' in each section when there are no services", () => {
    mockUseUsage.mockReturnValue(emptyState());

    render(<UsagePage />);

    const empties = screen.getAllByText("No usage recorded this month.");
    expect(empties).toHaveLength(4);
  });

  it("converts instance_seconds to hours (÷ 3600) in the Compute table", () => {
    mockUseUsage.mockReturnValue(dataState());

    render(<UsagePage />);

    // 7200 seconds → 2.00 hours
    expect(screen.getAllByText("2.00").length).toBeGreaterThanOrEqual(1);
    // 3600 seconds → 1.00 hours
    expect(screen.getAllByText("1.00").length).toBeGreaterThanOrEqual(1);
  });

  it("shows the tier/plan in the Compute table", () => {
    mockUseUsage.mockReturnValue(dataState());

    render(<UsagePage />);

    expect(screen.getByText("starter")).toBeInTheDocument();
    expect(screen.getByText("hobby")).toBeInTheDocument();
  });

  it("labels App, Postgres, and Key Value compute rows by resource kind", () => {
    mockUseUsage.mockReturnValue(dataState());

    render(<UsagePage />);

    expect(screen.getAllByText("service")).toHaveLength(2);
    expect(screen.getAllByText("postgres").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("key_value").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("shared")).toHaveLength(4);
  });

  it("shows the service name, falling back to the id when the name is empty", () => {
    mockUseUsage.mockReturnValue(dataState());

    render(<UsagePage />);

    // Named service renders its display name (compute + bandwidth + build rows),
    // never the srv- id.
    expect(screen.getAllByText("eden-cms-v2")).toHaveLength(3);
    expect(screen.queryByText("srv-cms")).not.toBeInTheDocument();
    // A service without a resolved name falls back to its id.
    expect(screen.getByText("email-worker")).toBeInTheDocument();
  });

  it("converts egress_bytes to MB in the Bandwidth table", () => {
    mockUseUsage.mockReturnValue(dataState());

    render(<UsagePage />);

    // 524288000 bytes → 500.00 MB; appears in both the service row and the Total row
    const cells = screen.getAllByText("500.00 MB");
    expect(cells.length).toBeGreaterThanOrEqual(1);
  });

  it("converts build_seconds to minutes (÷ 60) in the Build Minutes table", () => {
    mockUseUsage.mockReturnValue(dataState());

    render(<UsagePage />);

    // 1800 seconds → 30.0 minutes; appears in both the service row and the Total row
    const cells = screen.getAllByText("30.0");
    expect(cells.length).toBeGreaterThanOrEqual(1);
  });

  it("converts storage_gb_seconds to GB-hours and labels datastore kinds", () => {
    mockUseUsage.mockReturnValue(dataState());

    render(<UsagePage />);

    expect(screen.getByText("3.00")).toBeInTheDocument();
    expect(screen.getAllByText("postgres").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("key_value").length).toBeGreaterThanOrEqual(1);
  });

  it("renders Total rows summing each section", () => {
    mockUseUsage.mockReturnValue(dataState());

    render(<UsagePage />);

    const totals = screen.getAllByText("Total");
    expect(totals).toHaveLength(4);
  });

  it("renders the Estimated Cost section heading", () => {
    mockUseUsage.mockReturnValue(emptyState());

    render(<UsagePage />);

    expect(screen.getByText("Estimated Cost")).toBeInTheDocument();
  });

  it("shows the no-billable-usage empty state when estimatedCost is null", () => {
    // emptyState() has no estimatedCost field → null in the component
    mockUseUsage.mockReturnValue(emptyState());

    render(<UsagePage />);

    expect(
      screen.getByText("No billable usage this period."),
    ).toBeInTheDocument();
  });

  it("shows totalUsd and per-meter rows when estimatedCost is present", () => {
    mockUseUsage.mockReturnValue({
      ...dataState(),
      summary: {
        ...dataState().summary!,
        estimatedCost: {
          totalUsd: "4.90",
          meters: [
            {
              kind: "instance_seconds",
              tier: "starter",
              resourceKind: "service",
              costUsd: "4.90",
            },
          ],
        },
      },
    });

    render(<UsagePage />);

    // $4.90 appears in both the summary total span and the meter table cell
    expect(screen.getAllByText("$4.90").length).toBeGreaterThanOrEqual(2);
    expect(
      screen.getByText("Estimate only — not an invoice"),
    ).toBeInTheDocument();
    expect(screen.getByText("instance_seconds")).toBeInTheDocument();
  });

  it("renders the real Current Spend card (invoice, not estimate) when billing is present (m48)", () => {
    mockUseUsage.mockReturnValue({
      ...dataState(),
      summary: {
        ...dataState().summary!,
        billing: {
          currentCost: {
            amountUsd: "12.34",
            currency: "USD",
            periodStart: "2026-07-01T00:00:00Z",
            periodEnd: "2026-07-20T00:00:00Z",
          },
          invoices: [
            {
              id: "inv_1",
              status: "FINALIZED",
              amountUsd: "40.00",
              currency: "USD",
              periodStart: "2026-06-01T00:00:00Z",
              periodEnd: "2026-07-01T00:00:00Z",
            },
          ],
        },
      },
    });

    render(<UsagePage />);

    expect(screen.getByText("Current Spend")).toBeInTheDocument();
    expect(screen.getByText("$12.34")).toBeInTheDocument();
    expect(screen.getByText("$40.00")).toBeInTheDocument();
    expect(screen.getByText("FINALIZED")).toBeInTheDocument();
    // Distinguished as a real invoice, not an estimate.
    expect(screen.getByText("Invoice")).toBeInTheDocument();
  });

  it("omits the Current Spend card for an estimate-only workspace (no contract)", () => {
    // dataState() has no billing field → estimate-only; the card renders nothing.
    mockUseUsage.mockReturnValue(dataState());

    render(<UsagePage />);

    expect(screen.queryByText("Current Spend")).not.toBeInTheDocument();
  });

  it("shows an error alert when the query fails and no summary is available", () => {
    mockUseUsage.mockReturnValue({
      summary: null,
      loading: false,
      error: new Error("network error"),
    });

    render(<UsagePage />);

    expect(screen.getByText("Could not load usage")).toBeInTheDocument();
    expect(screen.getByText("network error")).toBeInTheDocument();
  });

  it("renders a month-picker select defaulting to current month", () => {
    mockUseUsage.mockReturnValue(emptyState());

    render(<UsagePage />);

    expect(screen.getByRole("combobox")).toBeInTheDocument();
    expect(screen.getByText("Current month")).toBeInTheDocument();
  });

  it("renders the 3-Month Trend section", () => {
    mockUseUsage.mockReturnValue(emptyState());

    render(<UsagePage />);

    expect(screen.getByText("3-Month Trend")).toBeInTheDocument();
    // Four trend charts — one per meter kind.
    expect(screen.getAllByTestId("svg-line-chart")).toHaveLength(4);
  });

  it("shows trend skeleton while trend data is loading", () => {
    mockUseUsage.mockReturnValue(emptyState());
    mockUseUsageTrend.mockReturnValue({ points: [], loading: true });

    const { container } = render(<UsagePage />);

    const skeletons = container.querySelectorAll(
      "[class*='animate-pulse'], [data-slot='skeleton']",
    );
    expect(skeletons.length).toBeGreaterThan(0);
  });
});

describe("credits (w5/m70)", () => {
  beforeEach(() => {
    mockUseUsageTrend.mockReturnValue(emptyTrendState());
  });

  function creditState(overrides?: {
    availableUsd?: string;
    expiresAt?: string;
  }) {
    return {
      summary: {
        workspaceId: "ws",
        period: "2026-07",
        services: [],
        estimatedCost: null,
        billing: {
          currentCost: {
            amountUsd: "12.34",
            currency: "USD",
            periodStart: "2026-08-16T00:00:00Z",
            periodEnd: "2026-09-16T00:00:00Z",
          },
          invoices: [],
          credits: {
            availableUsd: overrides?.availableUsd ?? "25.00",
            currency: "USD",
            grants: [
              {
                name: "welcome",
                remainingUsd: "20.00",
                expiresAt: overrides?.expiresAt ?? "2026-11-15T00:00:00Z",
              },
            ],
          },
        },
      },
      loading: false,
      error: undefined,
    };
  }

  it("hides every piece of credit chrome when the workspace holds none", () => {
    mockUseUsage.mockReturnValue(emptyState());

    render(<UsagePage />);

    expect(screen.queryByText("Credits remaining")).not.toBeInTheDocument();
    expect(screen.queryByText(/Credits applied/)).not.toBeInTheDocument();
  });

  it("renders balance, earliest expiry, applied line, and card-still-required copy", () => {
    mockUseUsage.mockReturnValue(creditState());

    render(<UsagePage />);

    expect(screen.getByText("Credits remaining")).toBeInTheDocument();
    expect(screen.getByText("$25.00")).toBeInTheDocument();
    expect(
      screen.getByText(/\$20\.00 of it expires 2026-11-15/),
    ).toBeInTheDocument();
    // Applied = min(available, current cost); due = remainder, floored at 0.
    expect(
      screen.getByText(/Credits applied −\$12\.34 → amount due \$0\.00/),
    ).toBeInTheDocument();
    // ADR046: credit never replaces payment onboarding.
    expect(
      screen.getByText(/payment method is still required even with credit/),
    ).toBeInTheDocument();
  });

  it("shows a positive amount due when credit only partially covers the period", () => {
    mockUseUsage.mockReturnValue(creditState({ availableUsd: "10.00" }));

    render(<UsagePage />);

    expect(
      screen.getByText(/Credits applied −\$10\.00 → amount due \$2\.34/),
    ).toBeInTheDocument();
  });

  it("omits the expiry note for never-expiring grants", () => {
    mockUseUsage.mockReturnValue(creditState({ expiresAt: "" }));

    render(<UsagePage />);

    expect(screen.getByText("Credits remaining")).toBeInTheDocument();
    expect(screen.queryByText(/of it expires/)).not.toBeInTheDocument();
  });
});

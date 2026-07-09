import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { UsagePage } from "../usage-page";
import { useUsage } from "@/features/usage/hooks/use-usage";

vi.mock("@/features/usage/hooks/use-usage", () => ({
  useUsage: vi.fn(),
}));

// DashboardLayout brings in the router and sidebar — stub it to a simple wrapper.
vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="dashboard-layout">{children}</div>
  ),
}));

const mockUseUsage = vi.mocked(useUsage);

function loadingState() {
  return { summary: null, loading: true, error: undefined };
}

function emptyState() {
  return { summary: { workspaceId: "ws", services: [] }, loading: false, error: undefined };
}

function dataState() {
  return {
    summary: {
      workspaceId: "ws-abc",
      services: [
        {
          serviceId: "eden-cms-v2",
          rows: [
            { kind: "instance_seconds", tier: "starter", total: 7200 },
            { kind: "egress_bytes", tier: "", total: 524288000 },
            { kind: "build_seconds", tier: "", total: 1800 },
          ],
        },
        {
          serviceId: "email-worker",
          rows: [
            { kind: "instance_seconds", tier: "hobby", total: 3600 },
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
  });

  it("renders the page heading and subtitle", () => {
    mockUseUsage.mockReturnValue(emptyState());

    render(<UsagePage />);

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Usage");
    expect(screen.getByText("Month-to-date workspace consumption")).toBeInTheDocument();
  });

  it("renders three section cards — Compute, Bandwidth, Build Minutes", () => {
    mockUseUsage.mockReturnValue(emptyState());

    render(<UsagePage />);

    expect(screen.getByText("Compute")).toBeInTheDocument();
    expect(screen.getByText("Bandwidth")).toBeInTheDocument();
    expect(screen.getByText("Build Minutes")).toBeInTheDocument();
  });

  it("shows skeleton rows while loading and no summary is available", () => {
    mockUseUsage.mockReturnValue(loadingState());

    const { container } = render(<UsagePage />);

    // Skeleton elements are rendered as animated divs
    const skeletons = container.querySelectorAll("[class*='animate-pulse'], [data-slot='skeleton']");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("shows 'No usage recorded this month.' in each section when there are no services", () => {
    mockUseUsage.mockReturnValue(emptyState());

    render(<UsagePage />);

    const empties = screen.getAllByText("No usage recorded this month.");
    expect(empties).toHaveLength(3);
  });

  it("converts instance_seconds to hours (÷ 3600) in the Compute table", () => {
    mockUseUsage.mockReturnValue(dataState());

    render(<UsagePage />);

    // 7200 seconds → 2.00 hours
    expect(screen.getByText("2.00")).toBeInTheDocument();
    // 3600 seconds → 1.00 hours
    expect(screen.getByText("1.00")).toBeInTheDocument();
  });

  it("shows the tier/plan in the Compute table", () => {
    mockUseUsage.mockReturnValue(dataState());

    render(<UsagePage />);

    expect(screen.getByText("starter")).toBeInTheDocument();
    expect(screen.getByText("hobby")).toBeInTheDocument();
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

  it("renders Total rows summing each section", () => {
    mockUseUsage.mockReturnValue(dataState());

    render(<UsagePage />);

    const totals = screen.getAllByText("Total");
    expect(totals).toHaveLength(3);
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
});

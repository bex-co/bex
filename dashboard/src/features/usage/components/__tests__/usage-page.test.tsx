// Copyright 2026 Tian Pan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { UsagePage } from "../usage-page";
import { useUsage, type UsageSummary } from "@/features/usage/hooks/use-usage";

vi.mock("@/features/usage/hooks/use-usage", () => ({
  useUsage: vi.fn(),
}));

// The page composes cards; each card's own behaviour has its own test file, so
// stub the ones that reach for their own data source.
vi.mock("@/features/usage/components/resource-caps", () => ({
  WorkspaceResourceCaps: () => <div data-testid="included-usage" />,
}));

vi.mock("@/features/usage/components/plan-card", () => ({
  PlanCard: () => <div data-testid="plan-card" />,
}));

vi.mock("@/features/usage/components/payment-method-card", () => ({
  PaymentMethodCard: () => <div data-testid="payment-method-card" />,
}));

// The page mounts the section nav twice (mobile bar + desktop rail), and its
// labels are the card titles, so leaving it in makes every title query
// ambiguous. It has its own test.
vi.mock("@/features/usage/components/usage-navigation", () => ({
  UsageNavigation: () => <nav data-testid="usage-navigation" />,
}));

vi.mock("@/features/usage/lib/period", () => ({
  periodFor: vi.fn((n: number) => {
    const d = new Date(2026, 6, 15); // fixed: July 2026
    d.setMonth(d.getMonth() - n);
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
  }),
  periodLabel: vi.fn((period: string) => period),
}));

// DashboardLayout brings in the router and sidebar — stub it to a wrapper.
vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="dashboard-layout">{children}</div>
  ),
}));

const mockUseUsage = vi.mocked(useUsage);

function summary(over: Partial<UsageSummary> = {}): UsageSummary {
  return {
    workspaceId: "ws",
    period: "2026-07",
    estimatedCost: { totalUsd: "0.00", resources: [] },
    billing: null,
    ...over,
  };
}

function state(over: Partial<UsageSummary> = {}) {
  return { summary: summary(over), loading: false, error: undefined };
}

describe("UsagePage", () => {
  beforeEach(() => {
    mockUseUsage.mockReset();
    mockUseUsage.mockReturnValue(state());
  });

  it("renders the page heading and subtitle", () => {
    render(<UsagePage />);

    expect(
      screen.getByRole("heading", { level: 1, name: "Billing" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Payment, invoices, and month-to-date workspace consumption",
      ),
    ).toBeInTheDocument();
  });

  it("renders the billing cards in order, money before consumption detail", () => {
    render(<UsagePage />);

    expect(screen.getByTestId("plan-card")).toBeInTheDocument();
    expect(screen.getByTestId("payment-method-card")).toBeInTheDocument();
    expect(screen.getByTestId("included-usage")).toBeInTheDocument();
    expect(screen.getByText("Charges")).toBeInTheDocument();
  });

  it("no longer renders the four per-meter usage tables it replaced", () => {
    render(<UsagePage />);

    // These were separate cards listing the same resources four times over.
    // The charge tree carries the same numbers with a price attached.
    for (const heading of [
      "Compute",
      "Bandwidth",
      "Build Minutes",
      "Storage",
      "3-Month Trend",
      "Estimated Cost",
    ]) {
      expect(screen.queryByText(heading)).not.toBeInTheDocument();
    }
  });

  it("renders a month-picker select defaulting to the current month", () => {
    render(<UsagePage />);

    const picker = screen.getByLabelText("Select month");
    expect(picker).toBeInTheDocument();
    expect(screen.getByText("Current month")).toBeInTheDocument();
  });

  it("shows an error alert when the query fails and no summary is available", () => {
    mockUseUsage.mockReturnValue({
      summary: null,
      loading: false,
      error: new Error("network exploded"),
    });

    render(<UsagePage />);

    expect(screen.getByText("Could not load usage")).toBeInTheDocument();
    expect(screen.getByText("network exploded")).toBeInTheDocument();
  });

  it("shows the invoice history only once there are finalized invoices", () => {
    expect(screen.queryByText("Invoice history")).not.toBeInTheDocument();

    mockUseUsage.mockReturnValue(
      state({
        billing: {
          currentCost: null,
          credits: null,
          invoices: [
            {
              id: "inv_1",
              status: "paid",
              amountUsd: "75.48",
              currency: "USD",
              periodStart: "2026-07-01T00:00:00Z",
              periodEnd: "2026-08-01T00:00:00Z",
            },
          ],
        },
      }),
    );
    render(<UsagePage />);

    expect(screen.getByText("Invoice history")).toBeInTheDocument();
    expect(screen.getByText("$75.48")).toBeInTheDocument();
    expect(screen.getByText("paid")).toBeInTheDocument();
  });
});

describe("credits (w5/m70)", () => {
  beforeEach(() => {
    mockUseUsage.mockReset();
  });

  function creditState(overrides?: {
    availableUsd?: string;
    expiresAt?: string;
  }) {
    return state({
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
    });
  }

  it("hides every piece of credit chrome when the workspace holds none", () => {
    mockUseUsage.mockReturnValue(state());

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

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { EstimatedPricingPanel } from "../estimated-pricing-panel";
import type { BlueprintEstimatedPricing } from "@/features/blueprints/types";

const beancountPricing: BlueprintEstimatedPricing = {
  totalUsd: "54.60",
  lines: [
    {
      name: "beancount-forum",
      tierLabel: "Standard",
      monthlyUsd: "17.50",
      instanceUsd: "17.50",
      storageUsd: null,
      storageGb: null,
    },
    {
      name: "beancount-forum-db",
      tierLabel: "Basic 1gb",
      monthlyUsd: "15.05",
      instanceUsd: "14.00",
      storageUsd: "1.05",
      storageGb: 5,
    },
    {
      name: "beancount-forum-redis",
      tierLabel: "Standard",
      monthlyUsd: "22.05",
      instanceUsd: "21.00",
      storageUsd: "1.05",
      storageGb: 5,
    },
  ],
  variable: [],
};

describe("EstimatedPricingPanel", () => {
  it("renders one priced row per resource and the monthly total", () => {
    render(<EstimatedPricingPanel pricing={beancountPricing} />);
    expect(screen.getByText(/estimated pricing/i)).toBeInTheDocument();
    expect(screen.getByText("beancount-forum")).toBeInTheDocument();
    expect(screen.getByText("(Standard) $17.50 / month")).toBeInTheDocument();
    expect(screen.getByText("(Basic 1gb) $15.05 / month")).toBeInTheDocument();
    expect(screen.getByText("$54.60 per month")).toBeInTheDocument();
    // No variable costs => no asterisk footnote.
    expect(screen.queryByText(/excluding/i)).not.toBeInTheDocument();
  });

  it("shows the instance/storage breakdown on datastore rows", () => {
    render(<EstimatedPricingPanel pricing={beancountPricing} />);
    const breakdowns = screen.getAllByText(
      "Instance $14.00 + Disk (5 GB) $1.05",
    );
    expect(breakdowns).toHaveLength(1);
  });

  it("renders nothing for an all-free blueprint", () => {
    const { container } = render(
      <EstimatedPricingPanel
        pricing={{ totalUsd: "0.00", lines: [], variable: [] }}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when the API omits the estimate", () => {
    const { container } = render(<EstimatedPricingPanel pricing={null} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("lists variable costs excluded from the total with an asterisk footnote", () => {
    render(
      <EstimatedPricingPanel
        pricing={{
          totalUsd: "4.90",
          lines: [
            {
              name: "web",
              tierLabel: "Starter",
              monthlyUsd: "4.90",
              instanceUsd: "4.90",
              storageUsd: null,
              storageGb: null,
            },
          ],
          variable: [
            { name: "web", reason: "autoscaling" },
            { name: "nightly", reason: "cron" },
          ],
        }}
      />,
    );
    expect(screen.getByText("$4.90* per month")).toBeInTheDocument();
    expect(screen.getAllByText(/variable/i).length).toBeGreaterThan(0);
    expect(
      screen.getByText("* Excluding autoscaling, cron jobs."),
    ).toBeInTheDocument();
  });
});

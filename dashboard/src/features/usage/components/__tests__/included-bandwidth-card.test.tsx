import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { IncludedBandwidthCard } from "@/features/usage/components/included-bandwidth-card";

// w6/m319: the plan's included outbound bandwidth, as Render's billing page shows.
describe("IncludedBandwidthCard", () => {
  it("shows bandwidth used against the plan's included bandwidth", () => {
    render(<IncludedBandwidthCard bandwidth={{ used: 1.234, included: 5 }} />);
    expect(
      screen.getByText("1.23 of 5 GB included bandwidth used this month"),
    ).toBeInTheDocument();
    expect(screen.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "25",
    );
    expect(screen.queryByText(/spin/)).toBeNull();
  });

  it("keeps a few MB from reading as nothing", () => {
    render(<IncludedBandwidthCard bandwidth={{ used: 0.003, included: 5 }} />);
    expect(
      screen.getByText("<0.01 of 5 GB included bandwidth used this month"),
    ).toBeInTheDocument();
  });

  it("warns near the allowance and says services spin down once spent without a card", () => {
    const { rerender } = render(
      <IncludedBandwidthCard bandwidth={{ used: 4.2, included: 5 }} />,
    );
    expect(
      screen.getByText(/close to your included bandwidth/),
    ).toBeInTheDocument();
    rerender(<IncludedBandwidthCard bandwidth={{ used: 5, included: 5 }} />);
    expect(
      screen.getByText(/included bandwidth is spent.*spun down/),
    ).toBeInTheDocument();
  });

  it("renders nothing without a plan allowance", () => {
    const { container } = render(
      <IncludedBandwidthCard bandwidth={{ used: 3, included: 0 }} />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});

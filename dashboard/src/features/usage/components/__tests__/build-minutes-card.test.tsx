import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { BuildMinutesCard } from "@/features/usage/components/build-minutes-card";

// w6/m308: the plan's included pipeline minutes, as Render's billing page shows.
describe("BuildMinutesCard", () => {
  it("shows build minutes used against the plan's included minutes", () => {
    render(<BuildMinutesCard minutes={{ used: 120, included: 500 }} />);
    expect(
      screen.getByText("120 of 500 included build minutes used this month"),
    ).toBeInTheDocument();
    expect(screen.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "24",
    );
    expect(screen.queryByText(/stop/)).toBeNull();
  });

  it("warns near the included minutes and says builds stop once spent without a card", () => {
    const { rerender } = render(
      <BuildMinutesCard minutes={{ used: 420, included: 500 }} />,
    );
    expect(
      screen.getByText(/close to your included build minutes/),
    ).toBeInTheDocument();
    rerender(<BuildMinutesCard minutes={{ used: 500, included: 500 }} />);
    expect(
      screen.getByText(/included build minutes are spent/),
    ).toBeInTheDocument();
  });

  it("renders nothing without a plan allowance", () => {
    const { container } = render(
      <BuildMinutesCard minutes={{ used: 30, included: 0 }} />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});

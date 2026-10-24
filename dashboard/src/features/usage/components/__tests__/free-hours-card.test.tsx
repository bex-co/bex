import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { FreeHoursCard } from "@/features/usage/components/free-hours-card";

// w8/m139: Render's "Monthly included usage" for Free web services.
describe("FreeHoursCard", () => {
  it("shows hours used against the allowance", () => {
    render(<FreeHoursCard hours={{ used: 120.5, included: 750 }} />);
    expect(
      screen.getByText("120.5 of 750 Free instance hours used this month"),
    ).toBeInTheDocument();
    expect(screen.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "16",
    );
    expect(screen.queryByText(/suspended/)).toBeNull();
  });

  it("warns near the allowance and says services are suspended once spent", () => {
    const { rerender } = render(
      <FreeHoursCard hours={{ used: 620, included: 750 }} />,
    );
    expect(screen.getByText(/close to the allowance/)).toBeInTheDocument();
    rerender(<FreeHoursCard hours={{ used: 750, included: 750 }} />);
    expect(screen.getByText(/The allowance is spent/)).toBeInTheDocument();
  });

  it("renders nothing without usage", () => {
    const { container } = render(<FreeHoursCard hours={null} />);
    expect(container).toBeEmptyDOMElement();
  });
});

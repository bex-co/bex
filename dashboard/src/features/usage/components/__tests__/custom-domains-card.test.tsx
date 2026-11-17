import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { CustomDomainsCard } from "@/features/usage/components/custom-domains-card";

// w7/m429: the plan's included custom domains as "N of M included", with the
// domains beyond them billed per domain-month and never refused.
describe("CustomDomainsCard", () => {
  it("shows the verified domains against the plan's included count", () => {
    render(
      <CustomDomainsCard
        domains={{ used: 1, included: 2, unlimited: false }}
      />,
    );
    expect(screen.getByText("Custom domains")).toBeInTheDocument();
    expect(
      screen.getByText("1 of 2 included custom domains in use"),
    ).toBeInTheDocument();
    expect(screen.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "50",
    );
    expect(screen.queryByText(/billed per domain each month\./)).toBeNull();
  });

  it("notes the count near it and says domains beyond it are billed, never refused", () => {
    const { rerender } = render(
      <CustomDomainsCard
        domains={{ used: 12, included: 15, unlimited: false }}
      />,
    );
    expect(
      screen.getByText(/close to your plan's included custom domains/),
    ).toBeInTheDocument();
    rerender(
      <CustomDomainsCard
        domains={{ used: 3, included: 2, unlimited: false }}
      />,
    );
    expect(
      screen.getByText("3 of 2 included custom domains in use"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /included custom domains are in use\. Each further verified domain is billed/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/never refused/)).toBeInTheDocument();
  });

  it("renders nothing without a count: a past month, no plan, or Enterprise", () => {
    for (const domains of [
      null,
      { used: 40, included: 0, unlimited: true },
      { used: 3, included: 0, unlimited: false },
    ]) {
      const { container, unmount } = render(
        <CustomDomainsCard domains={domains} />,
      );
      expect(container).toBeEmptyDOMElement();
      unmount();
    }
  });
});

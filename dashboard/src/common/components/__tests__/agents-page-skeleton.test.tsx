import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { AgentsPageSkeleton } from "@/common/components/detail-skeletons";

describe("AgentsPageSkeleton", () => {
  it("matches the default composer-only workspace", () => {
    const { container } = render(<AgentsPageSkeleton />);
    expect(screen.getByTestId("agents-page-skeleton")).toBeInTheDocument();
    expect(
      container.querySelector('[data-skeleton-region="composer"]'),
    ).toBeInTheDocument();
    expect(
      container.querySelector('[data-skeleton-region="session-list"]'),
    ).toBeNull();
    expect(
      container.querySelector('[data-skeleton-region="composer-hint"]'),
    ).toBeInTheDocument();
    expect(container.querySelector(".lg\\:grid-cols-3")).toBeNull();
  });

  it("sizes the prompt box to the measured toolbar wrap of the ready composer", () => {
    const { container } = render(<AgentsPageSkeleton />);
    const box = container.querySelector(
      '[data-skeleton-region="composer-box"]',
    );
    // Agent select + attachment chip + Start only (no Advanced trigger,
    // w10/m17): 3 toolbar rows below 345px, 2 below 469px, 1 above.
    expect(box).toHaveClass(
      "h-[14rem]",
      "min-[21.5625rem]:h-[11.75rem]",
      "min-[29.3125rem]:h-[9.5rem]",
    );
    expect(box?.className).not.toMatch(/\bsm:h-/);
  });

  it("matches filtered history without inventing the composer or tabs", () => {
    const { container } = render(<AgentsPageSkeleton mode="list" />);
    expect(
      container.querySelector('[data-skeleton-region="page-header"]'),
    ).toBeInTheDocument();
    expect(
      container.querySelector('[data-skeleton-region="session-list"]'),
    ).toBeInTheDocument();
    expect(
      container.querySelector('[data-skeleton-region="composer"]'),
    ).toBeNull();
  });
});

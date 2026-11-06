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
    // Agent select + attachment chip + Secrets + Skills + Start (no
    // Advanced trigger, w10/m17; Secrets ADR098; Skills ADR099): 3 toolbar
    // rows below 417px, 2 below 541px, 1 above.
    expect(box).toHaveClass(
      "h-[14rem]",
      "min-[26.0625rem]:h-[11.75rem]",
      "min-[33.8125rem]:h-[9.5rem]",
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

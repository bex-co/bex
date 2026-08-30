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

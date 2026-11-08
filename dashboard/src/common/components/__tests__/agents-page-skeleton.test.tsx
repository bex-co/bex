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

  // The toolbar wraps by the composer's own width, not the viewport: the
  // ready composer is capped at 40rem inside the app shell, so viewport
  // breakpoints predicted one row at desktop widths where it shows two (36px
  // drop on load, w9 browser check 2026-11-06). Same container classes and one
  // same-sized placeholder per control make it wrap like the ready toolbar.
  it("mirrors the ready composer's toolbar so it wraps at the same widths", () => {
    const { container } = render(<AgentsPageSkeleton />);
    const box = container.querySelector(
      '[data-skeleton-region="composer-box"]',
    );
    expect(box).toHaveClass("rounded-xl", "border");
    expect(box?.className).not.toMatch(/min-\[[^\]]+\]:h-/);
    expect(box?.firstElementChild).toHaveClass("min-h-28");
    const toolbar = container.querySelector(
      '[data-skeleton-region="composer-toolbar"]',
    );
    expect(toolbar).toHaveClass(
      "flex",
      "flex-wrap",
      "items-center",
      "gap-1",
      "px-1.5",
      "pb-1.5",
    );
    const controls = Array.from(toolbar?.children ?? []);
    // Agent select, attach, Secrets, Skills, spend limit, the repository
    // mention and Start: the ready toolbar's seven controls.
    expect(controls).toHaveLength(7);
    expect(controls[0]).toHaveClass("h-8", "w-[7.5rem]");
    for (const icon of controls.slice(1, 4)) expect(icon).toHaveClass("size-8");
    expect(controls[4]).toHaveClass("h-8", "w-28");
    // The text controls size to their real labels, drawn invisibly.
    expect(controls[5]).toHaveTextContent("Add repository or session");
    expect(controls[5]).toHaveClass(
      "max-w-56",
      "px-2",
      "text-xs",
      "text-transparent",
    );
    expect(controls[6]).toHaveClass("ml-auto", "h-8", "text-transparent");
    expect(controls[6]?.textContent?.length).toBeGreaterThan(0);
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

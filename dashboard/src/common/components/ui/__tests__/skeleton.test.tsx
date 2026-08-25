import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Skeleton } from "../skeleton";

describe("Skeleton", () => {
  it("should render with data-slot='skeleton'", () => {
    const { container } = render(<Skeleton />);

    const el = container.firstChild as HTMLElement;
    expect(el).toHaveAttribute("data-slot", "skeleton");
  });

  it("should have base classes: bg-accent, animate-pulse, rounded-md", () => {
    const { container } = render(<Skeleton />);

    const el = container.firstChild as HTMLElement;
    expect(el).toHaveClass("bg-accent");
    expect(el).toHaveClass("animate-pulse");
    expect(el).toHaveClass("motion-reduce:animate-none");
    expect(el).toHaveClass("group-data-[skeleton-frame=true]:!animate-none");
    expect(el).toHaveClass("rounded-md");
  });

  it("should accept and apply additional className", () => {
    const { container } = render(<Skeleton className="h-4 w-full" />);

    const el = container.firstChild as HTMLElement;
    expect(el).toHaveClass("h-4");
    expect(el).toHaveClass("w-full");
    // Base classes should still be present
    expect(el).toHaveClass("animate-pulse");
    expect(el).toHaveClass("rounded-md");
  });

  it("should pass through data-testid prop", () => {
    render(<Skeleton data-testid="my-skeleton" />);

    expect(screen.getByTestId("my-skeleton")).toBeInTheDocument();
  });

  it("should pass through aria-label prop", () => {
    render(<Skeleton aria-label="loading" />);

    expect(
      screen.getByRole("generic", { name: "loading" }),
    ).toBeInTheDocument();
  });

  it("should pass through other div props", () => {
    const { container } = render(<Skeleton id="skeleton-id" />);

    const el = container.firstChild as HTMLElement;
    expect(el).toHaveAttribute("id", "skeleton-id");
  });
});

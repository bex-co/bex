import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PageFrame, PageHeader } from "@/common/components/page-frame";

describe("PageFrame", () => {
  it("owns the scroll container and the width", () => {
    const { container } = render(
      <PageFrame width="form" header={<PageHeader title="New webhook" />}>
        <p>body</p>
      </PageFrame>,
    );
    const scroll = container.firstElementChild as HTMLElement;
    expect(scroll).toHaveAttribute("data-page-scroll");
    expect(scroll).toHaveClass("flex-1", "overflow-auto", "p-4", "sm:p-6");
    expect(scroll.querySelector(".max-w-2xl")).not.toBeNull();
    expect(
      screen.getByRole("heading", { level: 1, name: "New webhook" }),
    ).toBeInTheDocument();
  });

  it("puts a navigation in the sticky right rail beside the content", () => {
    const { container } = render(
      <PageFrame
        header={<PageHeader title="Billing" />}
        navigation={<nav aria-label="Sections" />}
      >
        <p>body</p>
      </PageFrame>,
    );
    const rail = screen.getByRole("navigation", {
      name: "Sections",
    }).parentElement!;
    // The rail form is chosen by the page container's width, not the
    // viewport's: the sidebar is resizable.
    expect(rail).toHaveClass(
      "sticky",
      "@4xl/page:col-start-2",
      "@4xl/page:row-span-2",
    );
    const scroller = container.firstElementChild as HTMLElement;
    expect(scroller).toHaveClass("@container/page");
    // The bar cancels the scroller's top padding to sit flush under the header.
    expect(scroller.className).toMatch(/\[--page-pad-top:1rem\]/);
    expect(rail.className).toMatch(/top-\[calc\(var\(--page-pad-top/);
    const grid = rail.parentElement!;
    // An explicit narrow column: an implicit `auto` one grows to the bar's
    // scroll width and pushes the page off-screen.
    expect(grid).toHaveClass("grid-cols-[minmax(0,1fr)]");
  });

  it("renders as an inert skeleton frame with real header copy when pending", () => {
    const { container } = render(
      <PageFrame pending="billing" header={<PageHeader title="Billing" />}>
        <div data-skeleton-region="plan" />
      </PageFrame>,
    );
    const frame = container.firstElementChild as HTMLElement;
    expect(frame).toHaveAttribute("data-route-skeleton", "billing");
    expect(frame).toHaveAttribute("data-skeleton-frame", "true");
    expect(frame).toHaveAttribute("aria-hidden", "true");
    expect(frame).toHaveAttribute("inert");
    const header = frame.querySelector('[data-skeleton-region="page-header"]');
    expect(header?.querySelector("h1")?.textContent).toBe("Billing");
    // The header holds still; only the body pulses.
    expect(header).not.toHaveClass("animate-pulse");
    expect(
      frame.querySelector('[data-skeleton-region="plan"]')?.parentElement,
    ).toHaveClass("animate-pulse");
  });

  it("lays out a header's leading control, description and actions", () => {
    render(
      <PageHeader
        leading={<button type="button">Back</button>}
        title="Env Groups"
        description="Share variables across services."
        actions={<button type="button">New</button>}
      />,
    );
    const header = screen
      .getByRole("heading", { level: 1 })
      .closest("[data-page-header]")!;
    expect(header).toHaveTextContent("Share variables across services.");
    expect(header.querySelectorAll("button")).toHaveLength(2);
  });
});

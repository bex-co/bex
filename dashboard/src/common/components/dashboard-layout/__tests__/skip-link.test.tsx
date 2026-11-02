import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { DashboardLayout } from "..";

vi.mock("../dashboard-sidebar", () => ({
  DashboardSidebar: () => <a href="/first-nav">Dashboard</a>,
}));
vi.mock("../dashboard-header", () => ({
  DashboardHeader: () => <button>Search</button>,
}));

// w4/460: WCAG 2.4.1 bypass block.
describe("DashboardLayout skip link", () => {
  it("is the first Tab stop and moves focus to the main landmark", async () => {
    const user = userEvent.setup();
    render(
      <DashboardLayout>
        <h1>Page</h1>
      </DashboardLayout>,
    );
    await user.tab();
    const skip = screen.getByRole("link", { name: "Skip to main content" });
    expect(skip).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(screen.getByRole("main")).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("main")).not.toHaveFocus();
  });

  it("is rendered once, even with a nested layout", () => {
    render(
      <DashboardLayout>
        <DashboardLayout>
          <h1>Page</h1>
        </DashboardLayout>
      </DashboardLayout>,
    );
    expect(
      screen.getAllByRole("link", { name: "Skip to main content" }),
    ).toHaveLength(1);
  });
});

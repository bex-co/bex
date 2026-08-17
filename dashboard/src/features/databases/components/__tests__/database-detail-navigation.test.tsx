import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DatabaseDetailNavigation } from "../database-detail-navigation";

describe("DatabaseDetailNavigation", () => {
  it("renders an accessible quick nav linking every overview section", () => {
    render(<DatabaseDetailNavigation className="sticky" />);

    const navigation = screen.getByRole("navigation", {
      name: "Database sections",
    });
    expect(navigation).toHaveClass("sticky");

    const expected: [string, string][] = [
      ["Details", "#metadata"],
      ["Connections", "#connection"],
      ["SQL console", "#sql-console"],
      ["High Availability", "#high-availability"],
      ["Metrics", "#metrics"],
      ["Instance type", "#plan"],
      ["Insights", "#insights"],
      ["Recovery", "#recovery"],
      ["Access control", "#access-control"],
      ["Danger Zone", "#danger-zone"],
    ];
    for (const [name, href] of expected) {
      expect(within(navigation).getByRole("link", { name })).toHaveAttribute(
        "href",
        href,
      );
    }
  });
});

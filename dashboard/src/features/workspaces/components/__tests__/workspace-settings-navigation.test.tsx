import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { WorkspaceSettingsNavigation } from "../workspace-settings-navigation";

describe("WorkspaceSettingsNavigation", () => {
  it("renders an accessible quick nav linking every settings section", () => {
    render(<WorkspaceSettingsNavigation className="sticky" showDangerZone />);

    const navigation = screen.getByRole("navigation", {
      name: "Settings sections",
    });
    expect(navigation).toHaveClass("sticky");

    const expected: [string, string][] = [
      ["General", "#general"],
      ["Team", "#team"],
      ["Danger Zone", "#danger-zone"],
    ];
    for (const [name, href] of expected) {
      expect(within(navigation).getByRole("link", { name })).toHaveAttribute(
        "href",
        href,
      );
    }
  });

  it("omits the danger-zone link when workspace deletion is unavailable", () => {
    render(<WorkspaceSettingsNavigation showDangerZone={false} />);

    const navigation = screen.getByRole("navigation", {
      name: "Settings sections",
    });
    expect(
      within(navigation).queryByRole("link", { name: "Danger Zone" }),
    ).not.toBeInTheDocument();
    expect(
      within(navigation).getByRole("link", { name: "General" }),
    ).toHaveAttribute("href", "#general");
    expect(
      within(navigation).getByRole("link", { name: "Team" }),
    ).toHaveAttribute("href", "#team");
  });
});

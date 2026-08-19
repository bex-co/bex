import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { WorkspaceSettingsNavigation } from "../workspace-settings-navigation";

describe("WorkspaceSettingsNavigation", () => {
  it("renders an accessible quick nav linking every settings section", () => {
    render(<WorkspaceSettingsNavigation className="sticky" />);

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
});

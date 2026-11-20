import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SettingsNavigation } from "../settings-navigation";

describe("SettingsNavigation", () => {
  it("links all six account settings sections, Appearance after Account, ending with Danger zone", () => {
    render(<SettingsNavigation className="sticky" />);
    const navigation = screen.getByRole("navigation", {
      name: "Settings sections",
    });
    const expected: [string, string][] = [
      ["Account", "#account"],
      ["Appearance", "#appearance"],
      ["Integrations", "#integrations"],
      ["Access credentials", "#access"],
      ["Security", "#security"],
      ["Danger zone", "#danger-zone"],
    ];
    for (const [name, href] of expected) {
      expect(within(navigation).getByRole("link", { name })).toHaveAttribute(
        "href",
        href,
      );
    }
  });
});

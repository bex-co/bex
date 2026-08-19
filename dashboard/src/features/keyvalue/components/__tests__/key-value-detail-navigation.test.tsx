import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { KeyValueDetailNavigation } from "../key-value-detail-navigation";

describe("KeyValueDetailNavigation", () => {
  it("renders an accessible quick nav linking every overview section", () => {
    render(<KeyValueDetailNavigation className="sticky" />);

    const navigation = screen.getByRole("navigation", {
      name: "Key Value sections",
    });
    expect(navigation).toHaveClass("sticky");

    const expected: [string, string][] = [
      ["Details", "#metadata"],
      ["Connections", "#connection"],
      ["Networking", "#networking"],
      ["Instance type", "#plan"],
      ["Maxmemory Policy", "#maxmemory-policy"],
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

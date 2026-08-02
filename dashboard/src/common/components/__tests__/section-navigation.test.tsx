import { render, screen, within } from "@testing-library/react";
import { KeyRound, UserRound } from "lucide-react";
import { describe, expect, it } from "vitest";
import { SectionNavigation } from "@/common/components/section-navigation";

describe("SectionNavigation", () => {
  it("renders an accessible list of in-page section links", () => {
    render(
      <SectionNavigation
        ariaLabel="Settings sections"
        className="sticky"
        items={[
          { href: "#account", label: "Account", icon: UserRound },
          {
            href: "#access-credentials",
            label: "Access credentials",
            icon: KeyRound,
          },
        ]}
      />,
    );

    const navigation = screen.getByRole("navigation", {
      name: "Settings sections",
    });
    expect(navigation).toHaveClass("sticky");
    expect(
      within(navigation).getByRole("link", { name: "Account" }),
    ).toHaveAttribute("href", "#account");
    expect(
      within(navigation).getByRole("link", { name: "Access credentials" }),
    ).toHaveAttribute("href", "#access-credentials");
    expect(navigation.querySelectorAll('svg[aria-hidden="true"]')).toHaveLength(
      2,
    );
  });
});

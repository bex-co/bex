import { render, screen, within } from "@testing-library/react";
import { KeyRound, UserRound } from "lucide-react";
import { describe, expect, it, vi } from "vitest";
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

describe("SectionNavigation scroll-spy", () => {
  function layout(el: HTMLElement, top: number, height: number) {
    el.getBoundingClientRect = () =>
      ({ top, height, width: 100, bottom: top + height }) as DOMRect;
  }

  it("marks the section the reader is in and drops a section that drew nothing", async () => {
    const { container } = render(
      <div data-page-scroll="">
        <SectionNavigation
          ariaLabel="Billing sections"
          items={[
            { href: "#plan", label: "Plan", icon: UserRound },
            { href: "#charges", label: "Charges", icon: KeyRound },
            { href: "#credits", label: "Credits", icon: KeyRound },
          ]}
        />
        <section id="plan" />
        <section id="charges" />
        <section id="credits" />
      </div>,
    );
    const scroller = container.firstElementChild as HTMLElement;
    layout(scroller, 0, 800);
    layout(screen.getByRole("navigation"), 0, 200);
    layout(container.querySelector("#plan")!, -400, 300);
    layout(container.querySelector("#charges")!, 40, 300);
    layout(container.querySelector("#credits")!, 400, 0);
    scroller.dispatchEvent(new Event("scroll"));

    const nav = screen.getByRole("navigation", { name: "Billing sections" });
    await vi.waitFor(() =>
      expect(
        within(nav).getByRole("link", { name: "Charges" }),
      ).toHaveAttribute("aria-current", "location"),
    );
    expect(within(nav).getByRole("link", { name: "Plan" })).not.toHaveAttribute(
      "aria-current",
    );
    expect(within(nav).queryByRole("link", { name: "Credits" })).toBeNull();
  });

  it("hides nothing without layout (server render, jsdom)", () => {
    render(
      <SectionNavigation
        ariaLabel="Sections"
        items={[{ href: "#missing", label: "Missing", icon: UserRound }]}
      />,
    );
    expect(screen.getByRole("link", { name: "Missing" })).toBeInTheDocument();
  });
});

describe("SectionNavigation forms", () => {
  it("styles every link for both the pill bar and the rail", () => {
    render(
      <SectionNavigation
        ariaLabel="Sections"
        items={[{ href: "#plan", label: "Plan", icon: UserRound }]}
      />,
    );
    const link = screen.getByRole("link", { name: "Plan" });
    // The bar's pill — filled when current, never an underline (a detail
    // page's own tabs are the underlined row above it).
    expect(link).toHaveClass(
      "rounded-full",
      "aria-[current=location]:bg-accent",
      "@4xl/page:rounded-md",
    );
    expect(link.className).not.toMatch(/after:/);
    // Icons only in the rail.
    expect(link.querySelector("svg")).toHaveClass("hidden", "@4xl/page:block");
    // The bar hides its scrollbar with the unlayered utility that out-ranks
    // the global `* { scrollbar-width: thin }`.
    expect(link.parentElement).toHaveClass("scrollbar-none", "overflow-x-auto");
  });

  it("scrolls smoothly to a clicked section and marks it current at once", async () => {
    const scrolled: ScrollIntoViewOptions[] = [];
    render(
      <div data-page-scroll="">
        <SectionNavigation
          ariaLabel="Sections"
          items={[
            { href: "#plan", label: "Plan", icon: UserRound },
            { href: "#charges", label: "Charges", icon: KeyRound },
          ]}
        />
        <section id="plan" />
        <section id="charges" />
      </div>,
    );
    document.getElementById("charges")!.scrollIntoView = (options) => {
      scrolled.push(options as ScrollIntoViewOptions);
    };
    screen.getByRole("link", { name: "Charges" }).click();
    expect(scrolled).toEqual([{ behavior: "smooth", block: "start" }]);
    expect(location.hash).toBe("#charges");
    await vi.waitFor(() =>
      expect(screen.getByRole("link", { name: "Charges" })).toHaveAttribute(
        "aria-current",
        "location",
      ),
    );
  });
});

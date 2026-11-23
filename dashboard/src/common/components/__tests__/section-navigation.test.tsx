import { fireEvent, render, screen, within } from "@testing-library/react";
import { KeyRound, UserRound } from "lucide-react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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

// w4/711: the router scrolls to a URL's hash once, as the page renders. Cards
// that load above the section afterwards (Workspace Settings' Agents cards
// shrink by about 2,000px) carried `#registry-credentials` off the screen, so
// each content resize re-aligns the section until the reader takes over.
describe("SectionNavigation holds the URL's section while the page loads", () => {
  const observers: { callback: () => void; targets: Element[] }[] = [];
  const OriginalResizeObserver = globalThis.ResizeObserver;

  beforeEach(() => {
    observers.length = 0;
    globalThis.ResizeObserver = class {
      private entry: { callback: () => void; targets: Element[] };
      constructor(callback: () => void) {
        this.entry = { callback, targets: [] };
        observers.push(this.entry);
      }
      observe(target: Element) {
        this.entry.targets.push(target);
      }
      unobserve() {}
      disconnect() {
        this.entry.targets = [];
      }
    } as unknown as typeof ResizeObserver;
  });

  afterEach(() => {
    globalThis.ResizeObserver = OriginalResizeObserver;
    history.replaceState(history.state, "", location.pathname);
    vi.useRealTimers();
  });

  /** The page's content changed size: run every observer watching it. */
  function contentResized(content: Element) {
    for (const { callback, targets } of observers) {
      if (targets.includes(content)) callback();
    }
  }

  function renderPage() {
    render(
      <div data-page-scroll="">
        <div data-testid="content">
          <SectionNavigation
            ariaLabel="Sections"
            items={[
              { href: "#plan", label: "Plan", icon: UserRound },
              { href: "#charges", label: "Charges", icon: KeyRound },
            ]}
          />
          <section id="plan" />
          <section id="charges" />
        </div>
      </div>,
    );
    const scrolled = vi.fn();
    document.getElementById("charges")!.scrollIntoView = scrolled;
    return { content: screen.getByTestId("content"), scrolled };
  }

  it("re-aligns the section on every content resize until the reader scrolls", () => {
    history.replaceState(history.state, "", "#charges");
    const { content, scrolled } = renderPage();

    contentResized(content);
    contentResized(content);
    expect(scrolled).toHaveBeenCalledTimes(2);
    expect(scrolled).toHaveBeenLastCalledWith({ block: "start" });

    fireEvent.wheel(window);
    contentResized(content);
    expect(scrolled).toHaveBeenCalledTimes(2);
  });

  it("lets go after five seconds", () => {
    vi.useFakeTimers();
    history.replaceState(history.state, "", "#charges");
    const { content, scrolled } = renderPage();

    vi.advanceTimersByTime(5000);
    contentResized(content);
    expect(scrolled).not.toHaveBeenCalled();
  });

  it("holds nothing without a hash", () => {
    const { content, scrolled } = renderPage();
    contentResized(content);
    expect(scrolled).not.toHaveBeenCalled();
  });
});

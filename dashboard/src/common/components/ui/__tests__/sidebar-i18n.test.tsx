import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "@/i18n/init";
import { MOBILE_BREAKPOINT } from "@/common/hooks/use-mobile";
import {
  Sidebar,
  SidebarContent,
  SidebarProvider,
  SidebarTrigger,
} from "../sidebar";

// The sidebar primitives name their own controls; those names follow the
// selected language like the links around them (w4/311).
function renderShell() {
  return render(
    <SidebarProvider>
      <Sidebar collapsible="icon">
        <SidebarContent>
          <a href="/">项目</a>
        </SidebarContent>
      </Sidebar>
      <SidebarTrigger />
    </SidebarProvider>,
  );
}

function setViewport(width: number) {
  Object.defineProperty(window, "innerWidth", {
    writable: true,
    configurable: true,
    value: width,
  });
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: (query: string) => ({
      matches: window.innerWidth < MOBILE_BREAKPOINT,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }),
  });
}

describe("Sidebar control labels follow the selected language (w4/311)", () => {
  const originalWidth = window.innerWidth;
  const originalMatchMedia = window.matchMedia;
  beforeEach(() => setViewport(1440));
  afterEach(async () => {
    await i18n.changeLanguage("en");
    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: originalWidth,
    });
    Object.defineProperty(window, "matchMedia", {
      configurable: true,
      writable: true,
      value: originalMatchMedia,
    });
  });

  it("names the desktop toggle and resize separator, and follows a switch", async () => {
    renderShell();
    expect(
      screen.getByRole("button", { name: "Toggle Sidebar" }),
    ).toBeInTheDocument();
    const rail = screen.getByRole("separator", { name: "Resize sidebar" });
    expect(rail).toHaveAttribute(
      "title",
      "Drag to resize · double-click to toggle",
    );

    await act(() => i18n.changeLanguage("zh"));
    expect(
      screen.getByRole("button", { name: "切换侧边栏" }),
    ).toBeInTheDocument();
    const zhRail = screen.getByRole("separator", { name: "调整侧边栏宽度" });
    expect(zhRail).toHaveAttribute("title", "拖动调整宽度 · 双击展开或收起");
    expect(zhRail).toHaveAttribute("aria-orientation", "vertical");
    expect(zhRail).toHaveAttribute(
      "aria-valuenow",
      rail.getAttribute("aria-valuenow"),
    );
  });

  it("names the mobile drawer, its description and toggle in Chinese", async () => {
    setViewport(390);
    await i18n.changeLanguage("zh");
    const user = userEvent.setup();
    renderShell();
    await user.click(screen.getByRole("button", { name: "切换侧边栏" }));
    const drawer = await screen.findByRole("dialog", { name: "侧边栏" });
    expect(drawer).toHaveAccessibleDescription("显示移动端侧边栏。");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

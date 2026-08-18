import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";

// The real cookie helper is an isomorphic fn that resolves to its server
// (no-op) variant under vitest — there is no request context — so back it with
// an in-memory store here. This lets the tests seed a persisted value and
// assert what the provider writes, exercising the read/persist wiring directly.
vi.mock("@/common/hooks/use-cookie-storage-state/cookie", () => {
  const store = new Map<string, string>();
  return {
    __cookieStore: store,
    getCookie: (key: string) => store.get(key),
    setCookie: (key: string, value: string) => store.set(key, String(value)),
    removeCookie: (key: string) => store.delete(key),
  };
});

import {
  getCookie,
  removeCookie,
  setCookie as seedCookie,
} from "@/common/hooks/use-cookie-storage-state/cookie";
import {
  Sidebar,
  SidebarContent,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
} from "../sidebar";
import {
  SIDEBAR_COOKIE_NAME,
  SIDEBAR_DEFAULT_WIDTH_PX,
  SIDEBAR_MAX_WIDTH_PX,
  SIDEBAR_MIN_WIDTH_PX,
} from "../sidebar-state";

// jsdom gives every element a zeroed getBoundingClientRect, so for the default
// left sidebar the drag math (clientX - wrapper.left) reduces to clientX — a
// pointerMove to clientX=N requests a width of N px.
function renderSidebar() {
  return render(
    <SidebarProvider>
      <Sidebar collapsible="icon">
        <SidebarContent>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton tooltip="Projects">
                <span>Projects</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarContent>
      </Sidebar>
    </SidebarProvider>,
  );
}

const sidebarEl = () =>
  document.querySelector('[data-slot="sidebar"]') as HTMLElement;
const wrapperWidth = () =>
  (
    document.querySelector('[data-slot="sidebar-wrapper"]') as HTMLElement
  ).style.getPropertyValue("--sidebar-width");
const persisted = () => getCookie(SIDEBAR_COOKIE_NAME);

describe("Sidebar resize + collapse (w2/m63)", () => {
  beforeEach(() => {
    removeCookie(SIDEBAR_COOKIE_NAME);
  });

  describe("persisted-state first paint (SSR read-back)", () => {
    it("renders collapsed at the remembered width from a signed cookie", () => {
      seedCookie(SIDEBAR_COOKIE_NAME, "-320");
      renderSidebar();
      expect(sidebarEl()).toHaveAttribute("data-state", "collapsed");
      // Icon-rail mode is engaged (the hook the label-hiding CSS keys off).
      expect(sidebarEl()).toHaveAttribute("data-collapsible", "icon");
      // Width memory is initialized even while collapsed, so re-expanding
      // restores it — and there is no width flash on hydration.
      expect(wrapperWidth()).toBe("320px");
    });

    it("renders expanded at the persisted width", () => {
      seedCookie(SIDEBAR_COOKIE_NAME, "300");
      renderSidebar();
      expect(sidebarEl()).toHaveAttribute("data-state", "expanded");
      expect(wrapperWidth()).toBe("300px");
      expect(screen.getByRole("separator")).toHaveAttribute(
        "aria-valuenow",
        "300",
      );
    });

    it("falls back to the default expanded width with no cookie", () => {
      renderSidebar();
      expect(sidebarEl()).toHaveAttribute("data-state", "expanded");
      expect(wrapperWidth()).toBe(`${SIDEBAR_DEFAULT_WIDTH_PX}px`);
    });
  });

  describe("drag to resize", () => {
    it("tracks the pointer within the band and commits + persists on release", () => {
      renderSidebar();
      const sep = screen.getByRole("separator");
      fireEvent.pointerDown(sep, { pointerId: 1, button: 0, clientX: 256 });
      fireEvent.pointerMove(sep, { pointerId: 1, clientX: 300 });
      // Live feedback: width var + aria-valuenow follow the pointer.
      expect(wrapperWidth()).toBe("300px");
      expect(sep).toHaveAttribute("aria-valuenow", "300");
      fireEvent.pointerUp(sep, { pointerId: 1, clientX: 300 });
      expect(wrapperWidth()).toBe("300px");
      expect(persisted()).toBe("300");
    });

    it("clamps a drag past the max width", () => {
      renderSidebar();
      const sep = screen.getByRole("separator");
      fireEvent.pointerDown(sep, { pointerId: 1, button: 0, clientX: 256 });
      fireEvent.pointerMove(sep, { pointerId: 1, clientX: 9999 });
      fireEvent.pointerUp(sep, { pointerId: 1, clientX: 9999 });
      expect(wrapperWidth()).toBe(`${SIDEBAR_MAX_WIDTH_PX}px`);
    });

    it("snaps to the collapsed rail when dragged below the threshold", () => {
      renderSidebar();
      const sep = screen.getByRole("separator");
      fireEvent.pointerDown(sep, { pointerId: 1, button: 0, clientX: 256 });
      fireEvent.pointerMove(sep, { pointerId: 1, clientX: 40 });
      fireEvent.pointerUp(sep, { pointerId: 1, clientX: 40 });
      expect(sidebarEl()).toHaveAttribute("data-state", "collapsed");
      // Negated remembered width persisted.
      expect(persisted()).toBe(`-${SIDEBAR_DEFAULT_WIDTH_PX}`);
    });

    it("re-expands when dragged back out from the collapsed rail", () => {
      seedCookie(SIDEBAR_COOKIE_NAME, "-256");
      renderSidebar();
      const sep = screen.getByRole("separator");
      expect(sidebarEl()).toHaveAttribute("data-state", "collapsed");
      fireEvent.pointerDown(sep, { pointerId: 1, button: 0, clientX: 48 });
      fireEvent.pointerMove(sep, { pointerId: 1, clientX: 300 });
      fireEvent.pointerUp(sep, { pointerId: 1, clientX: 300 });
      expect(sidebarEl()).toHaveAttribute("data-state", "expanded");
      expect(persisted()).toBe("300");
    });

    it("does not resize on a stationary click (leaves toggling to dblclick)", () => {
      renderSidebar();
      const sep = screen.getByRole("separator");
      fireEvent.pointerDown(sep, { pointerId: 1, button: 0, clientX: 256 });
      fireEvent.pointerUp(sep, { pointerId: 1, clientX: 257 });
      expect(sidebarEl()).toHaveAttribute("data-state", "expanded");
      expect(wrapperWidth()).toBe(`${SIDEBAR_DEFAULT_WIDTH_PX}px`);
    });
  });

  describe("toggle affordances", () => {
    it("double-click toggles expanded ⇄ collapsed", () => {
      renderSidebar();
      const sep = screen.getByRole("separator");
      fireEvent.doubleClick(sep);
      expect(sidebarEl()).toHaveAttribute("data-state", "collapsed");
      fireEvent.doubleClick(sep);
      expect(sidebarEl()).toHaveAttribute("data-state", "expanded");
    });

    it("Enter toggles and Arrow keys resize", () => {
      renderSidebar();
      const sep = screen.getByRole("separator");
      fireEvent.keyDown(sep, { key: "ArrowLeft" });
      expect(wrapperWidth()).toBe(`${SIDEBAR_DEFAULT_WIDTH_PX - 16}px`);
      expect(sep).toHaveAttribute(
        "aria-valuenow",
        `${SIDEBAR_DEFAULT_WIDTH_PX - 16}`,
      );
      fireEvent.keyDown(sep, { key: "Enter" });
      expect(sidebarEl()).toHaveAttribute("data-state", "collapsed");
    });
  });

  describe("ARIA window-splitter semantics", () => {
    it("exposes separator role + orientation + value range", () => {
      renderSidebar();
      const sep = screen.getByRole("separator");
      expect(sep).toHaveAttribute("aria-orientation", "vertical");
      expect(sep).toHaveAttribute("aria-valuemin", `${SIDEBAR_MIN_WIDTH_PX}`);
      expect(sep).toHaveAttribute("aria-valuemax", `${SIDEBAR_MAX_WIDTH_PX}`);
      expect(sep).toHaveAttribute(
        "aria-valuenow",
        `${SIDEBAR_DEFAULT_WIDTH_PX}`,
      );
    });
  });
});

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SidebarProvider, useSidebar } from "../sidebar";
import { MOBILE_BREAKPOINT } from "@/common/hooks/use-mobile";

// Test component that uses the useSidebar hook
function TestComponent() {
  const { isMobile, openMobile, setOpenMobile } = useSidebar();

  return (
    <div>
      <div data-testid="is-mobile">{isMobile ? "mobile" : "desktop"}</div>
      <div data-testid="open-mobile">{openMobile ? "open" : "closed"}</div>
      <button data-testid="set-open" onClick={() => setOpenMobile(true)}>
        Open
      </button>
      <button data-testid="set-closed" onClick={() => setOpenMobile(false)}>
        Close
      </button>
    </div>
  );
}

describe("Sidebar", () => {
  let originalMatchMedia: typeof window.matchMedia;

  beforeEach(() => {
    // Save original matchMedia
    originalMatchMedia = window.matchMedia;

    // Mock window.matchMedia
    const mockMatchMedia = (query: string) => {
      const matches = window.innerWidth < MOBILE_BREAKPOINT;
      const mql: MediaQueryList = {
        matches,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      };
      return mql;
    };

    Object.defineProperty(window, "matchMedia", {
      writable: true,
      configurable: true,
      value: mockMatchMedia,
    });
  });

  afterEach(() => {
    // Restore original matchMedia
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      configurable: true,
      value: originalMatchMedia,
    });
  });

  describe("useSidebar hook", () => {
    it("should export useSidebar hook", () => {
      expect(useSidebar).toBeDefined();
      expect(typeof useSidebar).toBe("function");
    });

    it("should provide sidebar context values", () => {
      render(
        <SidebarProvider>
          <TestComponent />
        </SidebarProvider>,
      );

      // Check that context values are available
      expect(screen.getByTestId("is-mobile")).toBeInTheDocument();
      expect(screen.getByTestId("open-mobile")).toBeInTheDocument();
    });

    it("should allow setting openMobile state", () => {
      render(
        <SidebarProvider>
          <TestComponent />
        </SidebarProvider>,
      );

      // Initially should be closed
      expect(screen.getByTestId("open-mobile")).toHaveTextContent("closed");

      // Click open button
      fireEvent.click(screen.getByTestId("set-open"));
      expect(screen.getByTestId("open-mobile")).toHaveTextContent("open");

      // Click close button
      fireEvent.click(screen.getByTestId("set-closed"));
      expect(screen.getByTestId("open-mobile")).toHaveTextContent("closed");
    });

    it("should throw error when used outside SidebarProvider", () => {
      // Suppress console.error for this test
      const consoleError = vi
        .spyOn(console, "error")
        .mockImplementation(() => {});

      expect(() => {
        render(<TestComponent />);
      }).toThrow("useSidebar must be used within a SidebarProvider.");

      consoleError.mockRestore();
    });
  });
});

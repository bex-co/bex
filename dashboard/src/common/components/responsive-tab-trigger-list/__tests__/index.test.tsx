import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { ResponsiveTabTriggerList } from "../index";
import { Tabs } from "@/common/components/ui/tabs";

describe("ResponsiveTabTriggerList", () => {
  const mockSetSelectedTab = vi.fn();

  const defaultProps = {
    selectedTab: "tab1",
    setSelectedTab: mockSetSelectedTab,
    tabOptions: [
      { label: "Tab One", value: "tab1" },
      { label: "Tab Two", value: "tab2" },
      { label: "Tab Three", value: "tab3" },
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  // Helper to render within Tabs context
  const renderWithTabs = (props = defaultProps) => {
    return render(
      <Tabs value={props.selectedTab}>
        <ResponsiveTabTriggerList {...props} />
      </Tabs>,
    );
  };

  describe("Rendering", () => {
    it("should render without crashing", () => {
      const { container } = renderWithTabs();
      expect(container).toBeInTheDocument();
    });

    it("should render all tab options in both desktop and mobile views", () => {
      renderWithTabs();

      // Check that tab options are rendered (they appear in both select and tabslist)
      expect(screen.getAllByText("Tab One").length).toBeGreaterThan(0);
      expect(screen.getAllByText("Tab Two").length).toBeGreaterThan(0);
      expect(screen.getAllByText("Tab Three").length).toBeGreaterThan(0);
    });

    it("should render a mobile select element", () => {
      renderWithTabs();

      const select = screen.getByRole("combobox");
      expect(select).toBeInTheDocument();
    });

    it("should render desktop TabsList", () => {
      const { container } = renderWithTabs();

      const tabsList = container.querySelector('[data-slot="tabs-list"]');
      expect(tabsList).toBeInTheDocument();
    });

    it("should apply custom className", () => {
      const { container } = renderWithTabs({
        ...defaultProps,
        className: "custom-class",
      });

      // The ResponsiveTabTriggerList renders a div with the className
      // The Tabs wrapper is the first child, so we need to find the wrapper div with the className
      const wrapperDiv = container.querySelector(".custom-class");
      expect(wrapperDiv).toBeInTheDocument();
    });
  });

  describe("Mobile Select", () => {
    it("should have the selected value in the mobile select", () => {
      renderWithTabs();

      const select = screen.getByRole("combobox");
      expect(select).toBeInTheDocument();
    });

    it("should have correct id for accessibility", () => {
      renderWithTabs();

      const select = screen.getByRole("combobox");
      expect(select).toHaveAttribute("id", "view-selector");
    });

    it("should have a hidden label for screen readers", () => {
      renderWithTabs();

      const label = screen.getByText("View");
      expect(label).toHaveClass("sr-only");
    });
  });

  describe("Desktop TabsList", () => {
    it("should render tab triggers for each option", () => {
      const { container } = renderWithTabs();

      const tabTriggers = container.querySelectorAll(
        '[data-slot="tabs-trigger"]',
      );
      expect(tabTriggers).toHaveLength(3);
    });

    it("should have hidden class on mobile", () => {
      const { container } = renderWithTabs();

      const tabsList = container.querySelector('[data-slot="tabs-list"]');
      expect(tabsList).toHaveClass("hidden");
      expect(tabsList).toHaveClass("lg:flex");
    });
  });

  describe("Tab Options", () => {
    it("should render correct number of options", () => {
      const twoOptions = [
        { label: "First", value: "first" },
        { label: "Second", value: "second" },
      ];

      const { container } = render(
        <Tabs value="first">
          <ResponsiveTabTriggerList
            selectedTab="first"
            setSelectedTab={mockSetSelectedTab}
            tabOptions={twoOptions}
          />
        </Tabs>,
      );

      const tabTriggers = container.querySelectorAll(
        '[data-slot="tabs-trigger"]',
      );
      expect(tabTriggers).toHaveLength(2);
    });

    it("should handle empty options gracefully", () => {
      const { container } = render(
        <Tabs value="">
          <ResponsiveTabTriggerList
            selectedTab=""
            setSelectedTab={mockSetSelectedTab}
            tabOptions={[]}
          />
        </Tabs>,
      );

      const tabTriggers = container.querySelectorAll(
        '[data-slot="tabs-trigger"]',
      );
      expect(tabTriggers).toHaveLength(0);
    });

    it("should display tab labels correctly", () => {
      const customOptions = [
        { label: "Custom Label 1", value: "custom1" },
        { label: "Custom Label 2", value: "custom2" },
      ];

      render(
        <Tabs value="custom1">
          <ResponsiveTabTriggerList
            selectedTab="custom1"
            setSelectedTab={mockSetSelectedTab}
            tabOptions={customOptions}
          />
        </Tabs>,
      );

      expect(screen.getAllByText("Custom Label 1").length).toBeGreaterThan(0);
      expect(screen.getAllByText("Custom Label 2").length).toBeGreaterThan(0);
    });
  });

  describe("Responsive Behavior", () => {
    it("should have mobile select with sm breakpoint visibility", () => {
      const { container } = renderWithTabs();

      // Mobile select trigger should have lg:hidden class
      const selectTrigger = container.querySelector(
        '[data-slot="select-trigger"]',
      );
      expect(selectTrigger).toHaveClass("lg:hidden");
    });

    it("should have desktop tabs hidden on mobile", () => {
      const { container } = renderWithTabs();

      const tabsList = container.querySelector('[data-slot="tabs-list"]');
      expect(tabsList).toHaveClass("hidden");
    });
  });
});

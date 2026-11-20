import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/common/components/ui/tooltip";
import { ThemeScope } from "../theme-scope";

// A subtree with a theme of its own (w3/m531): its menus, selects, popovers
// and tooltips portal out of it, and keep its theme, as Render's floating
// panels follow the log explorer's.

function Floating() {
  return (
    <>
      <DropdownMenu open>
        <DropdownMenuTrigger>menu</DropdownMenuTrigger>
        <DropdownMenuContent data-testid="menu">
          <DropdownMenuItem>Copy logs</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <Popover open>
        <PopoverTrigger>popover</PopoverTrigger>
        <PopoverContent data-testid="popover" aria-label="range">
          range
        </PopoverContent>
      </Popover>
      <Select open value="a">
        <SelectTrigger aria-label="select">
          <SelectValue />
        </SelectTrigger>
        <SelectContent data-testid="select">
          <SelectItem value="a">A</SelectItem>
        </SelectContent>
      </Select>
      <TooltipProvider>
        <Tooltip open>
          <TooltipTrigger>tip</TooltipTrigger>
          <TooltipContent data-testid="tooltip">hint</TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </>
  );
}

const PANELS = ["menu", "popover", "select", "tooltip"];

describe("ThemeScope (w3/m531)", () => {
  it("keeps its theme on itself and on everything it portals out", () => {
    const { container } = render(
      <ThemeScope theme="dark high-contrast" data-testid="scope">
        <Floating />
      </ThemeScope>,
    );
    const scope = screen.getByTestId("scope");
    expect(container.contains(scope)).toBe(true);
    expect(scope).toHaveClass("dark", "high-contrast", "text-foreground");
    for (const panel of PANELS) {
      const element = screen.getByTestId(panel);
      // Portalled: outside the scope's own subtree.
      expect(scope.contains(element)).toBe(false);
      expect(element).toHaveClass("dark", "high-contrast");
    }
  });

  it("follows the page, and adds nothing, with no theme of its own", () => {
    render(
      <ThemeScope theme={undefined} data-testid="scope">
        <Floating />
      </ThemeScope>,
    );
    expect(screen.getByTestId("scope").className).toBe("");
    for (const panel of PANELS) {
      const element = screen.getByTestId(panel);
      expect(element).not.toHaveClass("dark");
      expect(element).not.toHaveClass("light");
    }
  });

  it("passes an enclosing scope's theme through one with none", () => {
    render(
      <ThemeScope theme="light">
        <ThemeScope theme={undefined}>
          <Floating />
        </ThemeScope>
      </ThemeScope>,
    );
    expect(screen.getByTestId("menu")).toHaveClass("light");
  });
});

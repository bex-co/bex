import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import {
  SessionPrStateFilter,
  SessionPrStateFilterSkeleton,
} from "@/features/agent-sessions/components/session-pr-state-filter";

// Each option is a link to /agents; the mock exposes the search it carries.
vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    to,
    search,
    className,
    ...rest
  }: {
    children: React.ReactNode;
    to: string;
    search?: unknown;
    className?: string;
    "aria-current"?: "true";
  }) => (
    <a
      href={to}
      className={className}
      aria-current={rest["aria-current"]}
      data-search={JSON.stringify(search)}
    >
      {children}
    </a>
  ),
}));

function option(name: string): HTMLElement {
  return within(
    screen.getByRole("group", { name: "Filter by pull request state" }),
  ).getByRole("link", { name });
}

function searchOf(name: string): unknown {
  return JSON.parse(option(name).getAttribute("data-search") ?? "null");
}

describe("SessionPrStateFilter", () => {
  // w9/m386: one state at a time, written to the URL like the list's other
  // filters, keeping whichever of those are set.
  it("links each option to the list with that PR state, keeping the other filters", () => {
    render(<SessionPrStateFilter search={{ archived: "all", tag: "ci" }} />);
    expect(option("Open")).toHaveAttribute("href", "/agents");
    expect(searchOf("Open")).toEqual({
      archived: "all",
      tag: "ci",
      prState: "open",
    });
    expect(searchOf("Merged")).toEqual({
      archived: "all",
      tag: "ci",
      prState: "merged",
    });
    expect(searchOf("Closed")).toEqual({
      archived: "all",
      tag: "ci",
      prState: "closed",
    });
    // Any drops the PR-state filter (JSON drops the undefined key).
    expect(searchOf("Any")).toEqual({ archived: "all", tag: "ci" });
  });

  it("marks the active state, and Any when no PR state is set", () => {
    const { rerender } = render(<SessionPrStateFilter search={{}} />);
    expect(option("Any")).toHaveAttribute("aria-current", "true");
    expect(option("Merged")).not.toHaveAttribute("aria-current");

    rerender(<SessionPrStateFilter search={{ prState: "merged" }} />);
    expect(option("Merged")).toHaveAttribute("aria-current", "true");
    expect(option("Any")).not.toHaveAttribute("aria-current");
    expect(option("Open")).not.toHaveAttribute("aria-current");
  });

  it("drops the PR state, not the other filters, from Any", () => {
    render(
      <SessionPrStateFilter
        search={{ archived: "archived", phase: "failed", prState: "closed" }}
      />,
    );
    expect(searchOf("Any")).toEqual({ archived: "archived", phase: "failed" });
  });
});

describe("SessionPrStateFilterSkeleton", () => {
  // The pending list reserves the control at its real size: the same box
  // around the same labels, made invisible, so the swap does not shift.
  it("draws the ready control's box and labels invisibly", () => {
    const { container } = render(
      <>
        <div data-testid="ready">
          <SessionPrStateFilter search={{}} />
        </div>
        <div data-testid="pending">
          <SessionPrStateFilterSkeleton />
        </div>
      </>,
    );
    const ready = within(screen.getByTestId("ready")).getByRole("group");
    const pending = container.querySelector(
      '[data-skeleton-region="pr-state-filter"]',
    ) as HTMLElement;
    expect(pending).toHaveClass("text-transparent");
    for (const box of ["flex", "h-7", "shrink-0", "rounded-md", "p-0.5"]) {
      expect(ready).toHaveClass(box);
      expect(pending).toHaveClass(box);
    }
    const readyOptions = Array.from(ready.querySelectorAll("a"));
    const pendingOptions = Array.from(pending.querySelectorAll("span"));
    expect(pendingOptions.map((el) => el.textContent)).toEqual(
      readyOptions.map((el) => el.textContent),
    );
    expect(pendingOptions.map((el) => el.textContent)).toEqual([
      "Any",
      "Open",
      "Merged",
      "Closed",
    ]);
    pendingOptions.forEach((el, i) => {
      for (const size of ["h-6", "px-2", "text-xs", "font-medium"]) {
        expect(el).toHaveClass(size);
        expect(readyOptions[i]).toHaveClass(size);
      }
    });
    // The icon keeps its slot in both.
    expect(ready.querySelector("svg")).toHaveClass("mx-1", "size-3.5");
    expect(pending.querySelector("svg")).toHaveClass("mx-1", "size-3.5");
  });
});

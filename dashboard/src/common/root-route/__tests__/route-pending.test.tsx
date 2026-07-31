import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import RoutePending from "../route-pending";

describe("RoutePending", () => {
  // The default pending fallback replaces whatever route content suspends. If
  // it renders no DOM (as the old title-only pending component did), every
  // navigation slow enough to show it unmounts the page into a blank white
  // document — the regression this component exists to prevent.
  it("renders a visible loading indicator, not just a document title", () => {
    const { container } = render(<RoutePending />);
    const status = screen.getByRole("status");
    expect(status).toBeInTheDocument();
    expect(status).toHaveAccessibleName("Loading…");
    expect(container.querySelector(".animate-spin")).not.toBeNull();
  });
});

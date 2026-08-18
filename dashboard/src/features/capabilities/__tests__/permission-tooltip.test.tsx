import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";

describe("PermissionTooltip", () => {
  it("renders the child unwrapped when there is no reason (allowed)", () => {
    const { container } = render(
      <PermissionTooltip>
        <button>Reveal</button>
      </PermissionTooltip>,
    );
    expect(screen.getByRole("button", { name: "Reveal" })).toBeInTheDocument();
    // No tooltip trigger wrapper when the control is allowed.
    expect(
      container.querySelector('[data-slot="tooltip-trigger"]'),
    ).toBeNull();
  });

  it("wraps a disabled child in a tooltip trigger when a reason is given", () => {
    const { container } = render(
      <PermissionTooltip reason="Your role can’t do that">
        <button disabled>Reveal</button>
      </PermissionTooltip>,
    );
    // The disabled button still renders, and a hover/focus target wraps it so
    // the reason is reachable even though the button swallows pointer events.
    expect(screen.getByRole("button", { name: "Reveal" })).toBeDisabled();
    expect(
      container.querySelector('[data-slot="tooltip-trigger"]'),
    ).not.toBeNull();
  });
});

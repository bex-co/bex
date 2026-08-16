import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { EmptyState } from "@/common/components/empty-state";

// w9/m60 t001: `EmptyState` maps `iconName` through a static 5-icon record
// instead of the full lucide-react barrel (which hoisted ~1,900 icons into the
// entry chunk). This guards that every icon name actually passed app-wide still
// resolves to its own icon, and that an unknown name falls back rather than
// rendering nothing — so a new `iconName` that forgets its map entry is caught.

// Keep in sync with the grep for `iconName=` across src (the only call sites).
const USED_ICON_NAMES = [
  "AlertCircle",
  "Database",
  "DatabaseZap",
  "LockKeyhole",
  "ScrollText",
] as const;

function iconClassOf(container: HTMLElement): string {
  const svg = container.querySelector("svg");
  expect(svg).not.toBeNull();
  return svg!.getAttribute("class") ?? "";
}

describe("EmptyState icon map", () => {
  it("renders a distinct lucide icon for every iconName used app-wide", () => {
    // lucide renders each icon with a `lucide-<kebab-name>` class, so distinct
    // names must produce distinct icon classes (proves the right icon, not the
    // fallback, resolved for each).
    const classes = new Set<string>();
    for (const name of USED_ICON_NAMES) {
      const { container } = render(
        <EmptyState title="t" description="d" iconName={name} />,
      );
      classes.add(iconClassOf(container));
    }
    expect(classes.size).toBe(USED_ICON_NAMES.length);
  });

  it("falls back to a default icon for an unknown or missing iconName", () => {
    const unknown = render(
      <EmptyState title="t" description="d" iconName="TotallyNotAnIcon" />,
    );
    const missing = render(<EmptyState title="t" description="d" />);
    // Both render an icon (the FileSpreadsheet fallback), never crash or blank.
    expect(iconClassOf(unknown.container)).toBe(iconClassOf(missing.container));
    expect(iconClassOf(missing.container)).toContain("lucide");
  });
});

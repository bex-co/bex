import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/common/components/ui/tabs";
import { DetailTabNav } from "@/common/components/detail-tab-nav";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { css } from "@/test/theme-tokens";

// Buttons, links and tabs draw their focus indicator off the control's edge,
// as Render's do (w3/113): a 2px outline 2px off a button or a link and 1px
// off a tab, so a gap of the surface separates it from the control. A flush
// ring met a primary button's fill of the same green with no edge between
// them. Fields keep their flush ring and list rows their inset one. jsdom has
// no layout, so this reads the classes each control renders and the
// stylesheet's rules; the measured gap is in the item's record.

const tokens = (el: Element) => el.className.split(/\s+/);

/** The focus indicator a control's classes ask for. */
function indicator(el: Element) {
  const list = tokens(el);
  const offset = list
    .map((t) => /^focus-visible:(-?)outline-offset-(\d+)$/.exec(t))
    .find(Boolean);
  return {
    width: list.includes("focus-visible:outline-2"),
    offset: offset ? Number(`${offset[1]}${offset[2]}`) : 0,
    colour: list.filter((t) =>
      /^focus-visible:outline-(?!offset-)[a-z]/.test(t),
    ),
    ring: list.filter((t) => /^focus-visible:ring-(\d|\[)/.test(t)),
    // Tailwind's `outline-2` draws `var(--tw-outline-style)`, which an
    // unconditional `outline-none` or `outline-hidden` sets to none.
    voided: list.some((t) => t === "outline-none" || t === "outline-hidden"),
  };
}

describe("buttons, links and tabs draw their focus outline off their edge (w3/113)", () => {
  it.each([
    "default",
    "destructive",
    "outline",
    "secondary",
    "ghost",
    "link",
  ] as const)(
    "the %s button: a 2px outline 2px off, not a flush ring",
    (variant) => {
      render(<Button variant={variant}>Go</Button>);
      const focus = indicator(screen.getByRole("button"));
      expect(focus).toMatchObject({
        width: true,
        offset: 2,
        ring: [],
        voided: false,
      });
      // One colour wins the merge: the destructive button's own, else --ring.
      expect(focus.colour).toEqual([
        variant === "destructive"
          ? "focus-visible:outline-destructive"
          : "focus-visible:outline-ring",
      ]);
    },
  );

  it("a button rendered as a link keeps the button's outline", () => {
    render(
      <Button asChild>
        <a href="/x">Open</a>
      </Button>,
    );
    expect(indicator(screen.getByRole("link"))).toMatchObject({
      width: true,
      offset: 2,
      ring: [],
      voided: false,
    });
  });

  it("a badge that is a link draws the same outline", () => {
    render(
      <Badge asChild>
        <a href="/x">Live</a>
      </Badge>,
    );
    expect(indicator(screen.getByRole("link"))).toMatchObject({
      width: true,
      offset: 2,
      ring: [],
      voided: false,
    });
  });

  it.each(["default", "underline"] as const)(
    "the %s tab: a 2px outline 1px off, and its panel 2px off",
    (variant) => {
      render(
        <Tabs defaultValue="a">
          <TabsList variant={variant}>
            <TabsTrigger value="a">A</TabsTrigger>
          </TabsList>
          <TabsContent value="a">Panel</TabsContent>
        </Tabs>,
      );
      expect(indicator(screen.getByRole("tab"))).toMatchObject({
        width: true,
        offset: 1,
        ring: [],
        voided: false,
      });
      // Radix makes the panel a tab stop; it was `outline-none`, a stop
      // with no indicator at all.
      expect(indicator(screen.getByRole("tabpanel"))).toMatchObject({
        width: true,
        offset: 2,
        voided: false,
      });
    },
  );

  it("a resource detail page's tab draws the tabs' outline", () => {
    render(
      <DetailTabNav
        label="Views"
        tabs={[
          { id: "a", label: "Overview", active: true, onSelect: () => {} },
        ]}
      />,
    );
    expect(indicator(screen.getByRole("button"))).toMatchObject({
      width: true,
      offset: 1,
      voided: false,
    });
  });

  it("the focus stop that stands in for a disabled button draws the button's outline", () => {
    render(
      <PermissionTooltip reason="Admins only">
        <Button disabled>Delete</Button>
      </PermissionTooltip>,
    );
    const wrapper = screen.getByRole("button").parentElement!;
    expect(wrapper.tabIndex).toBe(0);
    expect(indicator(wrapper)).toMatchObject({
      width: true,
      offset: 2,
      voided: false,
    });
  });
});

/** The declarations of the first rule whose selector list is `selector`. */
function rule(selector: string): Record<string, string> {
  const start = css.indexOf(`${selector} {`);
  expect(start, `style.css has no rule for ${selector}`).toBeGreaterThan(-1);
  const body = css.slice(css.indexOf("{", start) + 1, css.indexOf("}", start));
  return Object.fromEntries(
    body
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .split(";")
      .map((d) => d.split(":").map((s) => s.trim()))
      .filter(([name, value]) => name && value),
  );
}

describe("the stylesheet's outline for links and buttons with no focus classes (w3/113)", () => {
  it("draws a solid 2px outline 2px off, in place of the browser's flush auto ring", () => {
    expect(rule("a[href]:focus-visible,\n  button:focus-visible")).toEqual({
      "outline-style": "solid",
      "outline-width": "2px",
      "outline-offset": "2px",
    });
  });

  it("scrolls a focused control far enough in that its outline is not cut by the edge", () => {
    // Tab scrolls a control just into view; the outline reaches 4px past it.
    const margin = rule("a[href],\n  button")["scroll-margin"];
    expect(margin).toMatch(/rem$/);
    expect(parseFloat(margin) * 16).toBeGreaterThanOrEqual(2 + 2);
  });

  it("draws the same on the sign-in pages' buttons and links, the show-password button inset", () => {
    expect(
      rule(".ory-elements :is(a[href], button:not([data-slot])):focus-visible"),
    ).toEqual({ outline: "2px solid var(--ring)", "outline-offset": "2px" });
    expect(
      rule(".ory-elements input + button:not([data-slot]):focus-visible"),
    ).toEqual({ "outline-offset": "-2px" });
  });
});

describe("a focus outline width is never voided by an unconditional outline-none", () => {
  // `outline-none focus-visible:outline-2` draws nothing in Tailwind 4: the
  // width utility's style is `var(--tw-outline-style)`, and `outline-none`
  // sets that to none on the element whatever its state. A class string
  // that asks for both needs `focus-visible:outline-solid` as well. This
  // reads one string literal at a time, so it cannot see classes a `cn()`
  // call joins from separate strings; the component tests above cover the
  // shared controls' merged classes.
  const sources = Object.entries(
    import.meta.glob<string>(
      ["/src/**/*.{ts,tsx}", "!/src/**/__tests__/**", "!/src/**/*.test.*"],
      { query: "?raw", import: "default", eager: true },
    ),
  );

  function voided(text: string): string[] {
    const found: string[] = [];
    for (const m of text.matchAll(/"([^"\n]*)"|`([^`]*)`|'([^'\n]*)'/g)) {
      const list = (m[1] ?? m[2] ?? m[3] ?? "").split(/\s+/);
      const none = list.some(
        (t) => t === "outline-none" || t === "outline-hidden",
      );
      const width = list.some((t) =>
        /^(?:[\w-]+:)*focus(?:-visible|-within)?:outline-(?:\d+|\[[\d.]+px\])$/.test(
          t,
        ),
      );
      const style = list.some((t) =>
        /^(?:[\w-]+:)*focus(?:-visible|-within)?:outline-(?:solid|dashed|dotted|double)$/.test(
          t,
        ),
      );
      if (none && width && !style) found.push(m[0]);
    }
    return found;
  }

  it("holds in every source file", () => {
    const found = sources.flatMap(([path, text]) =>
      voided(text).map((literal) => `${path}: ${literal}`),
    );
    expect(found, found.join("\n")).toEqual([]);
  });

  it("finds the pair it guards against", () => {
    expect(voided(`cn("outline-none focus-visible:outline-2")`)).toHaveLength(
      1,
    );
    expect(
      voided(
        `cn("outline-none focus-visible:outline-2 focus-visible:outline-solid")`,
      ),
    ).toEqual([]);
    expect(
      voided(`cn("focus-visible:outline-2 focus-visible:outline-offset-2")`),
    ).toEqual([]);
  });
});

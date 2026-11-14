import axe from "axe-core";

// w21/m395 t002: the accessibility pass over the rendered page-level fixtures.
//
// jsdom has no layout and no computed colour, so the rules that need either
// (`color-contrast`, the landmark and heading rules that judge a whole page,
// the experimental `label-content-name-mismatch`, which ends "incomplete" here)
// are not run: contrast is `theme-contrast.test.ts`, which tests the colour tokens, and the label
// check is `labelInNameViolations` below. What runs is the structural half,
// which is the class the board's accessible-name notes were: a control, dialog,
// image or field with no accessible name, an ARIA attribute or role that does
// not exist or is not allowed, a duplicate id behind an ARIA reference.
export const AXE_RULES = [
  "aria-allowed-attr",
  "aria-allowed-role",
  "aria-command-name",
  "aria-dialog-name",
  "aria-hidden-focus",
  "aria-input-field-name",
  "aria-meter-name",
  "aria-progressbar-name",
  "aria-required-attr",
  "aria-required-children",
  "aria-required-parent",
  "aria-roles",
  "aria-toggle-field-name",
  "aria-tooltip-name",
  "aria-treeitem-name",
  "aria-valid-attr",
  "aria-valid-attr-value",
  "button-name",
  "duplicate-id-aria",
  "empty-heading",
  "form-field-multiple-labels",
  "frame-title",
  "image-alt",
  "input-button-name",
  "input-image-alt",
  "label",
  "link-name",
  "list",
  "listitem",
  "nested-interactive",
  "role-img-alt",
  "select-name",
  "summary-name",
  "svg-img-alt",
  "tabindex",
] as const;

function describeNode(node: axe.NodeResult): string {
  const html = node.html.replace(/\s+/g, " ").slice(0, 160);
  return `${node.target.join(" ")}: ${html}`;
}

/** Axe's structural violations under `container`, one readable line each. */
export async function axeViolations(container: Element): Promise<string[]> {
  const results = await axe.run(container, {
    runOnly: { type: "rule", values: [...AXE_RULES] },
    resultTypes: ["violations"],
  });
  return results.violations.flatMap((violation) =>
    violation.nodes
      // While a modal is open, Radix hides the rest of the page from assistive
      // technology (`data-aria-hidden`) and traps focus, so the focusable
      // controls under it are unreachable, which is the point.
      .filter(
        (node) =>
          !(
            violation.id === "aria-hidden-focus" &&
            node.html.includes("data-aria-hidden")
          ),
      )
      .map(
        (node) => `${violation.id} (${violation.help}) — ${describeNode(node)}`,
      ),
  );
}

// WCAG 2.5.3 Label in Name: when a control shows text, its accessible name must
// contain that text, or a person who says what they see cannot activate it. The
// shape of w4/612: an `aria-label` that replaces the visible text.
// A combobox or listbox shows its current value, and its `aria-label` names the
// field, so the roles that take their name from their content are the ones
// checked (the set WCAG 2.5.3 applies to, and axe's own rule uses).
const NAMED_CONTROLS =
  'a[href], button:not([role="combobox"]):not([role="listbox"]), [role="button"], [role="link"], [role="menuitem"], [role="tab"], [role="checkbox"], [role="radio"], [role="switch"]';

function normalized(text: string): string {
  return text.replace(/\s+/g, " ").trim().toLowerCase();
}

function visibleText(element: Element): string {
  let text = "";
  for (const node of element.childNodes) {
    if (node.nodeType === Node.TEXT_NODE) text += node.textContent ?? "";
    else if (
      node instanceof Element &&
      node.getAttribute("aria-hidden") !== "true"
    ) {
      text += visibleText(node);
    }
  }
  return text;
}

/** Controls whose `aria-label` drops the text they display. */
export function labelInNameViolations(container: Element): string[] {
  const found: string[] = [];
  for (const element of container.querySelectorAll(NAMED_CONTROLS)) {
    const label = element.getAttribute("aria-label");
    if (!label) continue;
    const shown = normalized(visibleText(element));
    // Text with no letter or digit (an icon glyph, "×", "…") names nothing.
    if (!/[\p{L}\p{N}]/u.test(shown)) continue;
    if (normalized(label).includes(shown)) continue;
    found.push(
      `label-in-name (aria-label "${label}" omits the visible text "${shown}") — ${describeNode({ html: element.outerHTML, target: [element.tagName.toLowerCase()] } as axe.NodeResult)}`,
    );
  }
  return found;
}

/** Every accessibility violation under `container`. */
export async function a11yViolations(container: Element): Promise<string[]> {
  return [
    ...(await axeViolations(container)),
    ...labelInNameViolations(container),
  ];
}

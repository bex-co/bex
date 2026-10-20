/**
 * The card's structural signature: every element's tag and classes plus its
 * own static text, in document order. A `dd` collapses to its tag (its value
 * is resource data), and an input drops its value, so a preview matches the
 * ready card exactly when it has the same anatomy, copy and spacing.
 */
export function shape(element: Element): string {
  const tag = element.tagName.toLowerCase();
  if (tag === "dd") return "dd";
  if (tag === "svg") return `svg.${element.getAttribute("class")}`;
  const own = [...element.childNodes]
    .filter((node) => node.nodeType === Node.TEXT_NODE)
    .map((node) => node.textContent?.trim())
    .filter(Boolean)
    .join(" ");
  const children = [...element.children].map(shape).join(",");
  return `${tag}.${element.getAttribute("class") ?? ""}${own ? `"${own}"` : ""}[${children}]`;
}

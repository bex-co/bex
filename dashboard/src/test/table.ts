/** A rendered table's header labels, sr-only heads included. */
export function tableHeaders(root: ParentNode): (string | null)[] {
  return [...root.querySelectorAll("thead th")].map((th) => th.textContent);
}

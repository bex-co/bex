export type SearchableItem = { label: string; value: string };

// The name segment of a label — the part after the last "/", e.g. the repo name
// in "owner/repo". Falls back to the whole label when there is no slash.
function nameSegment(label: string): string {
  const slash = label.lastIndexOf("/");
  return (slash >= 0 ? label.slice(slash + 1) : label).toLowerCase();
}

// Filter by case-insensitive, trimmed substring, then sort **by repo name**:
// entries whose name segment matches the query rank above owner-only matches,
// then alphabetically by name. A blank query keeps every item, still name-sorted.
// Kept in a JSX-free module so the jest-lite runner can unit-test it.
export function filterItems<T extends SearchableItem>(
  items: T[],
  query: string,
): T[] {
  const q = query.trim().toLowerCase();
  const matched =
    q === ""
      ? items.slice()
      : items.filter((item) => item.label.toLowerCase().includes(q));
  return matched.sort((a, b) => {
    const an = nameSegment(a.label);
    const bn = nameSegment(b.label);
    if (q !== "") {
      const aName = an.includes(q) ? 0 : 1;
      const bName = bn.includes(q) ? 0 : 1;
      if (aName !== bName) return aName - bName;
    }
    return an.localeCompare(bn);
  });
}

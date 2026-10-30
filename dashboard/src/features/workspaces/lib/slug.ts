/**
 * The slug a freeform workspace name suggests: lowercase ASCII letters and
 * digits joined by single hyphens, at most 30 characters (the DNS-label rule).
 */
export function slugify(name: string): string {
  return name
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+/, "")
    .slice(0, 30)
    .replace(/-+$/, "");
}

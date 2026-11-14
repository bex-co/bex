/**
 * Identity of one service list page for hook memoization: the service plus the
 * cursors the page returned, joined with a NUL so no two distinct pages can
 * collapse to the same string. The env-vars and secret-files name hooks both
 * use it to tell a fresh first page (new cursors ⇒ re-anchor the tail) from a
 * poll that returned the same page.
 */
export function pageIdentity(
  serviceId: string,
  items: Array<{ cursor: string | null }>,
): string {
  return `${serviceId}\0${items.map((item) => item.cursor).join("\0")}`;
}

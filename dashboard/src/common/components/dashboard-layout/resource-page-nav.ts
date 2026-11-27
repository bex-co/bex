import type { SidebarNavGroup, SidebarNavItem } from "./sidebar-nav-groups";

/**
 * The page model a resource sidebar shares (w3/m641, w3/m649, w3/m677): a
 * fixed list of pages under one resource's URL, each a sidebar entry whose
 * label is also the page's heading.
 */
export interface ResourcePageNavItem extends SidebarNavItem {
  /**
   * Other paths under the resource on which this entry is the open page. One
   * ending in `/*` names every path below it, as Render's `hrefPatterns`
   * `/blueprint/:id/sync/:syncId` does (`/sync/*`).
   */
  alsoActiveOn: readonly string[];
}

function matchesPath(pattern: string, path: string): boolean {
  return pattern.endsWith("/*")
    ? path.startsWith(pattern.slice(0, -1))
    : pattern === path;
}

export function resourcePageNavItem(
  to: string,
  labelKey: SidebarNavItem["labelKey"],
  icon: SidebarNavItem["icon"],
  alsoActiveOn: readonly string[] = [],
): ResourcePageNavItem {
  return { labelKey, to, icon, alsoActiveOn };
}

/**
 * The open page of a resource whose pages live under `routeBase`
 * (`/databases/$databaseId`), drawn by `groups`: which entry is open, and
 * the open page's label.
 */
export function resourcePageNav(
  routeBase: string,
  groups: readonly SidebarNavGroup[],
) {
  const segment = routeBase.slice(0, routeBase.lastIndexOf("/"));

  /** The path under the resource that `pathname` names: `""` for the bare URL. */
  function pagePath(pathname: string, id: string): string | null {
    const base = `${segment}/${id}`;
    const path = pathname.replace(/\/+$/, "");
    if (path === base) return "";
    return path.startsWith(`${base}/`) ? path.slice(base.length) : null;
  }

  function itemFor(path: string): ResourcePageNavItem | undefined {
    for (const group of groups) {
      for (const item of group.items as ResourcePageNavItem[]) {
        if (
          item.to === `${routeBase}${path}` ||
          item.alsoActiveOn.some((pattern) => matchesPath(pattern, path))
        ) {
          return item;
        }
      }
    }
    return undefined;
  }

  return {
    /** Whether the sidebar entry `to` is the page `pathname` is on. */
    isActive(pathname: string, id: string, to: string): boolean {
      const path = pagePath(pathname, id);
      return path !== null && itemFor(path)?.to === to;
    },
    /**
     * The open page's sidebar label, which is also its heading, so the
     * loading frame and the loaded page carry the same words (on a phone the
     * sidebar is in a drawer, and nothing else on the page would say which
     * page is open). Null off the resource's pages.
     */
    labelKey(pathname: string, id: string): SidebarNavItem["labelKey"] | null {
      const path = pagePath(pathname, id);
      return path === null ? null : (itemFor(path)?.labelKey ?? null);
    },
  };
}

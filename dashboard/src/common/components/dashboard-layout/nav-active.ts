import { currentCreateType } from "@/common/components/dashboard-layout/create-type-items";

// "Projects" (to: "/") owns every route reachable from the Overview page or a
// project card — a project's own page, and each resource kind's list/detail/
// create routes — so it stays highlighted while browsing any of them, not
// just on the exact "/" path.
const PROJECTS_PREFIXES = ["/project/", "/services", "/databases", "/keyvalue"];
export function isNavItemActive(pathname: string, to: string): boolean {
  if (to === "/") {
    return (
      pathname === "/" ||
      // The create pages the Overview's New menu opens, at Render's URLs.
      currentCreateType(pathname) !== null ||
      PROJECTS_PREFIXES.some((p) => pathname.startsWith(p))
    );
  }
  return (
    pathname === to ||
    pathname.startsWith(`${to}/`) ||
    // Render's singular create URL sits under Blueprints.
    (to === "/blueprints" && pathname === "/blueprint/new")
  );
}

import { createFileRoute, redirect } from "@tanstack/react-router";
import { DatabaseInfoSkeleton } from "@/common/components/route-skeletons";
import { DatabaseInfoPage } from "@/features/databases/pages/database-info-page";
import { databasePageHead } from "@/features/databases/lib/database-page-head";
import { movedAnchorPage } from "@/features/databases/lib/database-pages";
import { databasePageRoute } from "@/features/databases/components/database-nav";

/**
 * The bare `/databases/<id>` shows Info, as Render's `/d/<id>` does (w3/m641).
 * It also takes the old single page's links to their page: `?tab=logs`, the
 * Logs tab (a project log view's View in context minted it, and bookmarks
 * keep it), with its range and filter keys; and an anchor of a card that left
 * Info (`#recovery`, `#plan`, …), on a client navigation. A full load's
 * anchor never reaches the server, so the Info page sends that one on itself.
 */
export const Route = createFileRoute("/databases/$databaseId/")({
  head: databasePageHead("databases.navInfo"),
  component: DatabaseInfoPage,
  pendingComponent: DatabaseInfoSkeleton,
  validateSearch: (search: Record<string, unknown>): { tab?: "logs" } => ({
    tab: search.tab === "logs" ? "logs" : undefined,
  }),
  beforeLoad: ({ params, search, location }) => {
    if (search.tab === "logs") {
      throw redirect({
        to: "/databases/$databaseId/logs",
        params,
        search: ({ tab: _tab, ...rest }: Record<string, unknown>) => rest,
        hash: location.hash || undefined,
        replace: true,
      });
    }
    const page = movedAnchorPage(location.hash);
    if (page) {
      throw redirect({
        to: databasePageRoute(page),
        params,
        hash: location.hash,
        replace: true,
      });
    }
  },
});

import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DatabaseCreatePageSkeleton } from "@/common/components/route-skeletons";
import { NewDatabasePage } from "@/features/databases/pages/new-database-page";

/**
 * Render's New-menu Postgres create URL (`/new/database`, live captures
 * 2026-07-16 and 2026-11-21) — a full page on the shared create template, as
 * on Render, which every Postgres create landing (`/d/new`, the overview's old
 * `?new=database`) now reaches (RENDER_CREATE_LANDINGS).
 */
export const Route = createFileRoute("/new/database")({
  staticData: { chrome: true },
  component: NewDatabasePage,
  pendingComponent: DatabaseCreatePageSkeleton,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("databases.createTitle", match),
});

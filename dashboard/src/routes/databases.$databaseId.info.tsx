import { createFileRoute } from "@tanstack/react-router";
import { DatabaseInfoSkeleton } from "@/common/components/route-skeletons";
import { DatabaseInfoPage } from "@/features/databases/pages/database-info-page";
import { databasePageHead } from "@/features/databases/lib/database-page-head";

/** Render's `/d/<id>/info`, the Info page its sidebar links (w3/m641). */
export const Route = createFileRoute("/databases/$databaseId/info")({
  head: databasePageHead("databases.navInfo"),
  component: DatabaseInfoPage,
  pendingComponent: DatabaseInfoSkeleton,
});

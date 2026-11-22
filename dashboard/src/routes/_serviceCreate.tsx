import { createFileRoute, useChildMatches } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { ServiceCreatePageSkeleton } from "@/common/components/route-skeletons";
import { NewServicePage } from "@/features/services/pages/new-service-page";
import {
  DEFAULT_SERVICE_TYPE,
  parseNewServiceSearch,
  serviceTypeFromCreatePath,
} from "@/features/services/lib/create-context";

/**
 * The service create page at Render's own URLs — `/web/new`, `/pserv/new`,
 * `/worker/new`, `/cron/new`, `/static/new` (live capture 2026-11-21) — one
 * pathless layout over the five, so the breadcrumb's type switch moves
 * between them without remounting the form (the draft survives, w6/045).
 * Each child route only names its path and its tab title; this layout renders
 * the page for the type its path names.
 */
export const Route = createFileRoute("/_serviceCreate")({
  staticData: { chrome: true },
  component: ServiceCreateLayout,
  pendingComponent: ServiceCreatePageSkeleton,
  beforeLoad: requireAuth(),
  validateSearch: parseNewServiceSearch,
});

function ServiceCreateLayout() {
  // The committed child's path, not `location`: the location moves as soon as
  // a navigation away starts, while this page is still on screen.
  const serviceType = useChildMatches({
    select: (matches) =>
      serviceTypeFromCreatePath(matches[0]?.pathname ?? "") ??
      DEFAULT_SERVICE_TYPE,
  });
  return (
    <NewServicePage serviceType={serviceType} search={Route.useSearch()} />
  );
}

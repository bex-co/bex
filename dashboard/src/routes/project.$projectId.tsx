import { createFileRoute, Outlet } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";

export const Route = createFileRoute("/project/$projectId")({
  component: RouteComponent,
  beforeLoad: requireAuth(),
});

/**
 * Shared chrome for every per-project page (Overview, Settings): just the
 * dashboard sidebar/header — DashboardSidebar swaps in the contextual
 * ProjectSidebar for any `/project/$projectId*` route, so no in-page tab nav
 * is needed here. Each child route (`.index.tsx`, `.settings.tsx`) owns its
 * own header content and content padding.
 */
function RouteComponent() {
  return (
    <DashboardLayout>
      <Outlet />
    </DashboardLayout>
  );
}

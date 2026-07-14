import { createFileRoute, Outlet, Link, useRouterState } from "@tanstack/react-router";
import { SearchX } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import { cn } from "@/common/lib/utils/utils.ts";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServer } from "@/features/services/hooks/use-server";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import {
  ServiceDetailHeader,
  ServiceDetailHeaderSkeleton,
} from "@/features/services/components/service-detail-header";
import { ServiceNav } from "@/features/services/components/service-nav";

export const Route = createFileRoute("/services/$serviceId")({
  component: RouteComponent,
  beforeLoad: requireAuth("/services/$serviceId"),
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceDetailLayout serviceId={serviceId} />;
}

/**
 * Shared chrome for every per-service page (Render's service-detail shape): the
 * overview header with lifecycle actions, the Overview/Logs nav, and an
 * `<Outlet/>` for the active tab. `server(id)` is read here for the header and
 * again in each child; Apollo's cache-and-network dedupes the two into one
 * request, so each route stays self-contained.
 */
export function ServiceDetailLayout({ serviceId }: { serviceId: string }) {
  const { service, loading, refetch } = useServer(serviceId);
  const { pending, run } = useServiceLifecycle({ refetch });
  const { t } = useTranslations();
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  // Render parity: the Settings tab's right-side quick-nav (captured live from
  // dashboard.render.com/web/.../settings) needs real width for its two-column
  // layout — every other tab keeps the narrower column, but flush-left like
  // Settings (not centered) so every tab's card text lines up with the
  // header's title, not just Settings'.
  const isSettingsTab = pathname.endsWith("/settings");

  // Unknown service id: `server(id)` resolved null and we're no longer loading.
  // Render a proper not-found state for the WHOLE detail (every tab) instead of
  // service chrome — never let a child tab borrow another service's data (the
  // 2026-07-09 phantom-service bug the fixed stub also guards against).
  if (!service && !loading) {
    return (
      <DashboardLayout>
        <div className="flex-1 overflow-auto py-4 sm:py-6">
          <div className="mx-auto w-full max-w-4xl">
            <Card>
              <CardContent className="flex flex-col items-center justify-center gap-4 py-16 text-center">
                <SearchX className="text-muted-foreground/50 h-8 w-8" />
                <div>
                  <p className="mb-1 font-medium">{t("services.notFoundTitle")}</p>
                  <p className="text-muted-foreground text-sm">
                    {t("services.notFoundBody", { name: serviceId })}
                  </p>
                </div>
                <Button asChild variant="outline" size="sm">
                  <Link to="/">{t("services.notFoundBackToList")}</Link>
                </Button>
              </CardContent>
            </Card>
          </div>
        </div>
      </DashboardLayout>
    );
  }

  return (
    <DashboardLayout>
      {service ? (
        <ServiceDetailHeader
          service={service}
          pending={pending?.id === service.id ? pending.action : null}
          onRun={run}
        />
      ) : (
        <ServiceDetailHeaderSkeleton name={serviceId} />
      )}
      <ServiceNav serviceId={serviceId} />
      {/* No horizontal padding here (only vertical) — every tab's content is a
          `Card`, whose own `px-6` header/content padding already indents its
          text; adding padding here too would double-indent it past the
          header's `eden-cms-v2` title. Card's *border* sits flush with this
          wrapper's edge, landing exactly under the header's own `px-4 sm:px-6`
          text at the `sm+` breakpoint (both text columns line up). */}
      <div className="flex-1 overflow-auto py-4 sm:py-6">
        <div
          className={cn(
            // Flush-left, not `mx-auto`-centered: centering would add its own
            // margin whenever max-w exceeds the (now padding-free) wrapper's
            // content width, undoing the card/header text alignment above on
            // any tab, not just Settings.
            "w-full space-y-6",
            isSettingsTab ? "max-w-6xl" : "max-w-4xl",
          )}
        >
          <Outlet />
        </div>
      </div>
    </DashboardLayout>
  );
}

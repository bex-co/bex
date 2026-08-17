import type { ReactNode } from "react";
import { Navigate } from "@tanstack/react-router";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServer } from "@/features/services/hooks/use-server";
import {
  useStaticSiteMutations,
  type UseStaticSiteMutationsResult,
} from "@/features/services/hooks/use-static-site";
import { isStaticSite } from "@/features/services/lib/service-type";
import type { ServiceView } from "@/features/services/types";
import type { en } from "@/i18n";

/**
 * Shared shell for the two dedicated static-site edge-rule pages (w5/m48/t002)
 * — Render puts Redirects/Rewrites and Headers on their own Manage pages
 * (`/static/<id>/redirects`, `/static/<id>/headers`), not inside Settings. The
 * shell owns the pieces both pages must agree on: the non-static guard (a
 * direct URL for any other type redirects to Settings — these rules only exist
 * for static sites), the skeleton-while-loading state, and the Card scaffold
 * (name-only header; the editors carry their own title + hint). The editors
 * render keyed by service id so their drafts reseed when navigating between
 * services. Rules apply live without a rebuild (w1/m21).
 */
export function StaticEdgeRulePage({
  serviceId,
  titleKey,
  renderEditor,
}: {
  serviceId: string;
  titleKey: keyof typeof en;
  renderEditor: (
    service: ServiceView,
    mutations: UseStaticSiteMutationsResult,
  ) => ReactNode;
}) {
  const { t } = useTranslations();
  const { service, loading, refetch } = useServer(serviceId, { poll: false });
  const mutations = useStaticSiteMutations(serviceId, refetch);

  if (service && !isStaticSite(service)) {
    return (
      <Navigate
        to="/services/$serviceId/settings"
        params={{ serviceId }}
        replace
      />
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t(titleKey)}</CardTitle>
      </CardHeader>
      <CardContent>
        {!service && loading ? (
          <Skeleton className="h-24 w-full" />
        ) : service ? (
          renderEditor(service, mutations)
        ) : null}
      </CardContent>
    </Card>
  );
}

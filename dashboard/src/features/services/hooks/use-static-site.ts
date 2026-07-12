import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  SetStaticRoutesDocument,
  SetStaticHeadersDocument,
  SetPublishPathDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import type {
  ServiceView,
  StaticRouteView,
  StaticHeaderView,
} from "@/features/services/types";

// A static_site's edge rules (routes/headers) and publish directory are
// App-CR spec fields (docs/static-sites.md). Routes/headers are read live by the
// shared static-server, so they apply without a rebuild; changing the publish
// path republishes. Each mutation refetches the service so the settings view
// reflects the new state, and toasts the result.

export interface UseStaticSiteMutationsResult {
  /** Replace the whole ordered redirect/rewrite list; true on success. */
  setRoutes: (routes: StaticRouteView[]) => Promise<boolean>;
  /** Replace the whole custom-header list; true on success. */
  setHeaders: (headers: StaticHeaderView[]) => Promise<boolean>;
  /** Change the published output directory (republishes); true on success. */
  setPublishPath: (publishPath: string) => Promise<boolean>;
  /** A write is in flight (disable the forms while true). */
  busy: boolean;
}

export function useStaticSiteMutations(
  serviceId: string,
  refetch: () => Promise<ServiceView[]>,
): UseStaticSiteMutationsResult {
  const { t } = useTranslations();
  const [setStaticRoutes] = useMutation(SetStaticRoutesDocument);
  const [setStaticHeaders] = useMutation(SetStaticHeadersDocument);
  const [setPublishPathMut] = useMutation(SetPublishPathDocument);
  const [busy, setBusy] = useState(false);

  const setRoutes = useCallback(
    async (routes: StaticRouteView[]) => {
      setBusy(true);
      try {
        await setStaticRoutes({ variables: { id: serviceId, routes } });
        await refetch();
        toast.success(t("services.staticRoutesSaved"));
        return true;
      } catch {
        toast.error(t("services.staticRoutesError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [serviceId, setStaticRoutes, refetch, t],
  );

  const setHeaders = useCallback(
    async (headers: StaticHeaderView[]) => {
      setBusy(true);
      try {
        await setStaticHeaders({ variables: { id: serviceId, headers } });
        await refetch();
        toast.success(t("services.staticHeadersSaved"));
        return true;
      } catch {
        toast.error(t("services.staticHeadersError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [serviceId, setStaticHeaders, refetch, t],
  );

  const setPublishPath = useCallback(
    async (publishPath: string) => {
      setBusy(true);
      try {
        await setPublishPathMut({ variables: { id: serviceId, publishPath } });
        await refetch();
        toast.success(t("services.publishPathSaved"), {
          description: t("services.publishPathRepublishNote"),
        });
        return true;
      } catch {
        toast.error(t("services.publishPathError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [serviceId, setPublishPathMut, refetch, t],
  );

  return { setRoutes, setHeaders, setPublishPath, busy };
}

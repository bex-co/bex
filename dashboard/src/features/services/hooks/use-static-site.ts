import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  SetStaticRoutesDocument,
  SetStaticHeadersDocument,
  SetPublishPathDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
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

/**
 * A bulk edge-rule save's outcome, including the rows the server ACCEPTED.
 *
 * The rows matter because the backend normalizes what it stores — it trims
 * route type/source/destination and header path/name (`apps/service.go`) — so a
 * padded path saves successfully as its canonical form. Reporting only
 * `boolean` left each editor showing the padded draft it submitted while the
 * stored configuration said something else, with Save still enabled and no way
 * to tell the change had already persisted (w4/136).
 *
 * `saved` is absent, never `[]`, when the post-save read did not produce this
 * service: an unreadable list must not be presented as an empty rule set.
 */
export interface StaticRuleSaveResult<T> {
  ok: boolean;
  saved?: T[];
}

export interface UseStaticSiteMutationsResult {
  /** Replace the whole ordered redirect/rewrite list; returns the accepted rows. */
  setRoutes: (
    routes: StaticRouteView[],
  ) => Promise<StaticRuleSaveResult<StaticRouteView>>;
  /** Replace the whole custom-header list; returns the accepted rows. */
  setHeaders: (
    headers: StaticHeaderView[],
  ) => Promise<StaticRuleSaveResult<StaticHeaderView>>;
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
        // The refetch already runs; this reads the canonical rows out of it
        // rather than discarding them. They come from the shared read mapping
        // (toServiceView), so no __typename or nullable member reaches a draft.
        const saved = (await refetch()).find(
          (service) => service.id === serviceId,
        );
        toast.success(t("services.staticRoutesSaved"));
        return saved ? { ok: true, saved: saved.routes } : { ok: true };
      } catch (err) {
        toast.error(mutationErrorMessage(err, t("services.staticRoutesError")));
        return { ok: false };
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
        const saved = (await refetch()).find(
          (service) => service.id === serviceId,
        );
        toast.success(t("services.staticHeadersSaved"));
        return saved ? { ok: true, saved: saved.headers } : { ok: true };
      } catch (err) {
        toast.error(
          mutationErrorMessage(err, t("services.staticHeadersError")),
        );
        return { ok: false };
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
      } catch (err) {
        toast.error(mutationErrorMessage(err, t("services.publishPathError")));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [serviceId, setPublishPathMut, refetch, t],
  );

  return { setRoutes, setHeaders, setPublishPath, busy };
}

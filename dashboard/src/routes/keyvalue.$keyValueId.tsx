import { cn } from "@/common/lib/utils/utils";
import { PageScroll } from "@/common/components/page-frame";
import {
  createFileRoute,
  useNavigate,
  useRouter,
} from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import {
  DatastoreLogsSkeleton,
  DatastoreMetricsSkeleton,
  KeyValueOverviewSkeleton,
} from "@/common/components/route-skeletons";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import {
  resourceLoadErrorVariant,
  resourceNotFound,
  useNotFoundRedirect,
} from "@/common/hooks/use-not-found-redirect";
import { DetailTabNav } from "@/common/components/detail-tab-nav";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { KeyValueMetadataCard } from "@/features/keyvalue/components/key-value-metadata-card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useKeyValue } from "@/features/keyvalue/hooks/use-key-value";
import { useKeyValueInstanceTypes } from "@/features/keyvalue/hooks/use-key-value-instance-types";
import type { KeyValueView } from "@/features/keyvalue/types";
import { useAdoptResourceWorkspace } from "@/features/workspaces/hooks/use-adopt-resource-workspace";
import { KeyValueStatusBadge } from "@/features/keyvalue/components/key-value-status-badge";
import { KeyValueDangerActions } from "@/features/keyvalue/components/key-value-danger-actions";
import { ConnectionInfoPanel } from "@/features/keyvalue/components/connection-info-panel";
import { KeyValueNetworkingPanel } from "@/features/keyvalue/components/key-value-networking-panel";
import { KeyValuePlanSection } from "@/features/keyvalue/components/key-value-plan-section";
import { KeyValueMaxmemoryPolicySection } from "@/features/keyvalue/components/key-value-maxmemory-policy-section";
import { KeyValuePersistenceModeSection } from "@/features/keyvalue/components/key-value-persistence-mode-section";
import { KeyValueDetailNavigation } from "@/features/keyvalue/components/key-value-detail-navigation";
import { DatastoreLogViewer } from "@/features/logs/components/datastore-log-viewer";
import {
  DEFAULT_RANGE_PRESET,
  rangeFromSearch,
  rangeToSearch,
  type RangeSelection,
} from "@/features/metrics/lib/range";
import {
  datastoreLogSearchProps,
  parseDatastoreLogSearch,
  type DatastoreLogSearch,
} from "@/features/logs/lib/datastore-log-search";
import { metricsRangeSearch } from "@/features/logs/lib/log-range";
import { DatastoreMetricsPanel } from "@/features/metrics/components/datastore-metrics-panel";
import { KeyValueDocument } from "@/graphql/definitions";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
} from "@/common/lib/document-head";
import {
  SECTIONED_BODY_CLASS,
  SECTIONED_GRID_CLASS,
  SECTION_NAVIGATION_STICKY_CLASS,
  SECTION_NAVIGATION_TARGET_CLASS,
} from "@/common/components/section-navigation";

export const Route = createFileRoute("/keyvalue/$keyValueId")({
  staticData: { chrome: true },
  component: KeyValueDetailPage,
  // The page doubles as its own pending state at 0ms: it renders full
  // chrome + its skeleton stack while its Apollo read loads (tolerating the
  // absent loaderData), so the title-loader wait shows the real frame
  // instead of the router-level blank that used to flash white.
  pendingComponent: KeyValueDetailPage,
  pendingMs: 0,
  // The pending frame is (or renders) the page itself: keep it in the page's
  // chunk instead of the default reference-file placement (vite.config.ts).
  codeSplitGroupings: [
    ["component", "pendingComponent"],
    ["errorComponent"],
    ["notFoundComponent"],
  ],
  beforeLoad: requireAuth(),
  validateSearch: (
    search: Record<string, unknown>,
  ): { tab?: "logs" | "metrics" } & DatastoreLogSearch => ({
    tab:
      search.tab === "logs" || search.tab === "metrics"
        ? search.tab
        : undefined,
    // The Logs and Metrics time ranges persist in the shared `range`/
    // `rangeStart`/`rangeEnd` URL shape (w6/065, w4/703), and the Logs
    // text/instance filters beside them (w4/228), so a filtered view survives
    // a reload and is shareable.
    ...parseDatastoreLogSearch(search),
  }),
  loader: ({ context, params, cause }) =>
    loadRouteResource(
      () =>
        context.client.query({
          query: KeyValueDocument,
          variables: { id: params.keyValueId },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
      (data) => (data?.keyValue?.name?.trim() ? data.keyValue : null),
    ),
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (keyValue) => [
        keyValue.name,
        translatedText("keyvalue.resourceType"),
      ]),
      match,
    ),
});

export function KeyValueDetailPage() {
  const { keyValueId } = Route.useParams();
  const search = Route.useSearch();
  const { tab } = search;
  const { t } = useTranslations();
  const navigate = useNavigate();
  const logsDenied = useCapabilities().denied("can_view_logs");
  const router = useRouter();
  // Functional form: committed verbatim, so a patch's explicit undefineds
  // actually clear stale keys (w7/m42); the spread keeps `tab` and the
  // untouched keys in place.
  const writeSearch = (patch: DatastoreLogSearch) =>
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, ...patch }),
      replace: true,
    });
  // The Metrics range lives in the same w6/065 URL keys as the Logs tab's, so
  // a picked window survives a reload and a copied link (w4/703). Each tab
  // link resets the search, so one tab's range never leaks into the other.
  const metricsRange = {
    range: rangeFromSearch(metricsRangeSearch(search), DEFAULT_RANGE_PRESET),
    onRangeChange: (range: RangeSelection) =>
      writeSearch(rangeToSearch(range)),
  };
  const { keyValue: loaded, loading, error, refetch } = useKeyValue(keyValueId);
  // Judge this page's controls in the store's own workspace (w4/641, as
  // w6/091 for services): until it is the selection, the skeleton stays up so
  // no role-gated control paints for the wrong workspace first.
  const adopting = useAdoptResourceWorkspace(keyValueId, loaded?.ownerId);
  const keyValue = adopting ? null : loaded;

  // A dead id redirects home (w9/m55); a failed query stays put on the inline
  // error state so an outage never masquerades as a deleted store. A
  // roll-window loader failure re-runs once (w1/m52) so the title recovers.
  useNotFoundRedirect(resourceNotFound(loaded, loading, error));
  useLoaderErrorRetry(Route.useLoaderData(), keyValueId);
  const loadErrorVariant = resourceLoadErrorVariant(loaded, loading, error);

  return (
    <DashboardLayout>
      <div
        className="contents"
        data-route-skeleton={!keyValue ? "keyvalue-detail" : undefined}
      >
        <div
          data-skeleton-region={!keyValue ? "resource-header" : undefined}
          className="flex flex-wrap items-center gap-3 border-b px-4 py-4 sm:px-6"
        >
          {keyValue ? (
            <div className="flex min-w-0 max-w-full items-center gap-2">
              <h1 className="truncate text-xl font-semibold">
                {keyValue.name}
              </h1>
              <KeyValueStatusBadge keyValue={keyValue} />
            </div>
          ) : (
            <div
              data-skeleton-region="resource-title"
              className="flex min-w-0 max-w-full items-center gap-2"
            >
              {/* The h1's text-xl line box is 28px (w4/m209). */}
              <Skeleton className="h-7 w-48" />
              <Skeleton className="h-5 w-16 rounded-full" />
            </div>
          )}
        </div>

        <DetailTabNav
          skeletonRegion={!keyValue ? "tabs" : undefined}
          label={t("keyvalue.detailNavLabel")}
          tabs={[
            {
              id: "overview",
              label: t("keyvalue.overviewTab"),
              active: !tab,
              onSelect: () => void navigate({ to: ".", search: {} }),
            },
            {
              id: "metrics",
              label: t("keyvalue.metricsTab"),
              active: tab === "metrics",
              onSelect: () =>
                void navigate({ to: ".", search: { tab: "metrics" } }),
            },
            // Neither a Viewer nor a Billing member views logs (w9/145):
            // the tab is left out, but a deep link to it still names the
            // open tab, over the role's reason.
            ...(logsDenied && tab !== "logs"
              ? []
              : [
                  {
                    id: "logs",
                    label: t("keyvalue.logsTab"),
                    active: tab === "logs",
                    onSelect: () =>
                      void navigate({ to: ".", search: { tab: "logs" } }),
                  },
                ]),
          ]}
        />

        <PageScroll data-skeleton-region={!keyValue ? "active-tab" : undefined}>
          {loadErrorVariant ? (
            <div className="mx-auto w-full max-w-5xl space-y-6">
              <ResourceLoadError
                onRetry={() => void refetch()}
                variant={loadErrorVariant}
              />
            </div>
          ) : keyValue && tab === "logs" ? (
            <div className="mx-auto w-full max-w-5xl space-y-6">
              <DatastoreLogViewer
                kind="keyvalue"
                resource={keyValue.id}
                {...datastoreLogSearchProps(search, writeSearch)}
              />
            </div>
          ) : keyValue && tab === "metrics" ? (
            <div className="mx-auto w-full max-w-5xl space-y-6">
              <KeyValueMetrics keyValue={keyValue} {...metricsRange} />
            </div>
          ) : keyValue ? (
            <div className={cn("mx-auto max-w-5xl", SECTIONED_GRID_CLASS)}>
              {/* Same right-rail quick nav as the service settings page. */}
              <KeyValueDetailNavigation
                className={SECTION_NAVIGATION_STICKY_CLASS}
              />

              <div className={SECTIONED_BODY_CLASS}>
                <section
                  id="metadata"
                  className={SECTION_NAVIGATION_TARGET_CLASS}
                >
                  <KeyValueMetadataCard
                    keyValue={keyValue}
                    onRenamed={() => void router.invalidate()}
                  />
                </section>
                <section
                  id="connection"
                  className={SECTION_NAVIGATION_TARGET_CLASS}
                >
                  {/* A revealed snapshot belongs to one publication and
                      internal-auth state: when a confirmed change flips
                      `public` or `internalAuth`, remount so the old URL is no
                      longer copyable and a fresh Reveal is required (w4/222).
                      Renames and status polls keep it. */}
                  <ConnectionInfoPanel
                    key={`${keyValue.id}:${keyValue.public}:${keyValue.internalAuth}`}
                    id={keyValue.id}
                    name={keyValue.name}
                    internalAuth={keyValue.internalAuth ?? false}
                    isPublic={keyValue.public}
                  />
                </section>
                <section
                  id="networking"
                  className={SECTION_NAVIGATION_TARGET_CLASS}
                >
                  <KeyValueNetworkingPanel id={keyValue.id} />
                </section>
                <section id="plan" className={SECTION_NAVIGATION_TARGET_CLASS}>
                  <KeyValuePlanSection
                    keyValue={keyValue}
                    onChanged={() => void refetch()}
                  />
                </section>
                <section
                  id="maxmemory-policy"
                  className={SECTION_NAVIGATION_TARGET_CLASS}
                >
                  <KeyValueMaxmemoryPolicySection
                    id={keyValue.id}
                    onChanged={() => void refetch()}
                  />
                </section>
                <section
                  id="persistence-mode"
                  className={SECTION_NAVIGATION_TARGET_CLASS}
                >
                  <KeyValuePersistenceModeSection
                    key={keyValueId}
                    id={keyValue.id}
                    name={keyValue.name}
                    onChanged={() => void refetch()}
                  />
                </section>
                <section
                  id="danger-zone"
                  className={SECTION_NAVIGATION_TARGET_CLASS}
                >
                  <KeyValueDangerActions
                    keyValue={keyValue}
                    onDeleted={() => void navigate({ to: "/", replace: true })}
                    onChanged={() => void refetch()}
                  />
                </section>
              </div>
            </div>
          ) : tab === "logs" ? (
            <DatastoreLogsSkeleton />
          ) : tab === "metrics" ? (
            <DatastoreMetricsSkeleton kind="keyvalue" />
          ) : (
            <KeyValueOverviewSkeleton />
          )}
        </PageScroll>
      </div>
    </DashboardLayout>
  );
}

/**
 * The Metrics tab. Render's Active Connections chart reads the plan's
 * connection limit beside its title (w6/m459), from the same cache-first
 * catalog the Overview's plan cards read.
 */
function KeyValueMetrics({
  keyValue,
  range,
  onRangeChange,
}: {
  keyValue: KeyValueView;
  range: RangeSelection;
  onRangeChange: (range: RangeSelection) => void;
}) {
  const { instanceTypes } = useKeyValueInstanceTypes();
  return (
    <DatastoreMetricsPanel
      kind="keyvalue"
      resource={keyValue.id}
      range={range}
      onRangeChange={onRangeChange}
      connectionLimit={
        instanceTypes.find((it) => it.id === keyValue.plan)?.connectionLimit
      }
    />
  );
}

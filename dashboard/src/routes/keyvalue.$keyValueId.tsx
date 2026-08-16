import {
  createFileRoute,
  useNavigate,
  useRouter,
} from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import { useNotFoundRedirect } from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import { MetadataList } from "@/common/components/metadata-list";
import {
  CardSkeleton,
  MetadataListSkeleton,
} from "@/common/components/detail-skeletons";
import { cn } from "@/common/lib/utils/utils.ts";
import { formatRelativeAge } from "@/features/services/lib/format";
import { useKeyValue } from "@/features/keyvalue/hooks/use-key-value";
import { KeyValueStatusBadge } from "@/features/keyvalue/components/key-value-status-badge";
import { KeyValueDangerActions } from "@/features/keyvalue/components/key-value-danger-actions";
import { ConnectionInfoPanel } from "@/features/keyvalue/components/connection-info-panel";
import { KeyValueNetworkingPanel } from "@/features/keyvalue/components/key-value-networking-panel";
import { KeyValuePlanSection } from "@/features/keyvalue/components/key-value-plan-section";
import { KeyValueMaxmemoryPolicySection } from "@/features/keyvalue/components/key-value-maxmemory-policy-section";
import { KeyValueNameRow } from "@/features/keyvalue/components/key-value-name-row";
import { KeyValueLogViewer } from "@/features/keyvalue/components/key-value-log-viewer";
import { DatastoreMetricsPanel } from "@/features/metrics/components/datastore-metrics-panel";
import type { KeyValueView } from "@/features/keyvalue/types";
import { KeyValueDocument } from "@/graphql/definitions";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
} from "@/common/lib/document-head";

export const Route = createFileRoute("/keyvalue/$keyValueId")({
  staticData: { chrome: true },
  component: KeyValueDetailPage,
  // The page doubles as its own pending state at 0ms: it renders full
  // chrome + its skeleton stack while its Apollo read loads (tolerating the
  // absent loaderData), so the title-loader wait shows the real frame
  // instead of the router-level blank that used to flash white.
  pendingComponent: KeyValueDetailPage,
  pendingMs: 0,
  beforeLoad: requireAuth(),
  validateSearch: (search: Record<string, unknown>): { tab?: "logs" } =>
    search.tab === "logs" ? { tab: "logs" } : {},
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
  const { tab } = Route.useSearch();
  const { t } = useTranslations();
  const navigate = useNavigate();
  const router = useRouter();
  const { keyValue, loading, error, refetch } = useKeyValue(keyValueId);

  // A dead id redirects home (w9/m55); a failed query stays put on the inline
  // error state so an outage never masquerades as a deleted store. A
  // roll-window loader failure re-runs once (w1/m52) so the title recovers.
  useNotFoundRedirect(!loading && !keyValue && !error);
  useLoaderErrorRetry(Route.useLoaderData(), keyValueId);
  const showError = !loading && !keyValue && !!error;

  return (
    <DashboardLayout>
      <div className="flex flex-wrap items-center gap-3 border-b px-4 py-4 sm:px-6">
        <div className="flex items-center gap-2">
          <h1
            className={cn(
              "truncate text-xl font-semibold",
              !keyValue && "text-muted-foreground",
            )}
          >
            {keyValue?.name ?? keyValueId}
          </h1>
          {keyValue ? <KeyValueStatusBadge keyValue={keyValue} /> : null}
        </div>
      </div>

      <nav
        aria-label={t("keyvalue.detailNavLabel")}
        className="flex gap-1 border-b px-4 sm:px-6"
      >
        <button
          type="button"
          className={cn(
            "border-b-2 px-3 py-2 text-sm",
            tab !== "logs"
              ? "border-foreground text-foreground"
              : "border-transparent text-muted-foreground",
          )}
          onClick={() => void navigate({ to: ".", search: {} })}
        >
          {t("keyvalue.overviewTab")}
        </button>
        <button
          type="button"
          className={cn(
            "border-b-2 px-3 py-2 text-sm",
            tab === "logs"
              ? "border-foreground text-foreground"
              : "border-transparent text-muted-foreground",
          )}
          onClick={() => void navigate({ to: ".", search: { tab: "logs" } })}
        >
          {t("keyvalue.logsTab")}
        </button>
      </nav>

      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          {showError ? (
            <ResourceLoadError onRetry={() => void refetch()} />
          ) : keyValue && tab === "logs" ? (
            <KeyValueLogViewer resource={keyValue.id} />
          ) : keyValue ? (
            <>
              <MetadataCard
                keyValue={keyValue}
                onRenamed={() => void router.invalidate()}
              />
              <ConnectionInfoPanel id={keyValue.id} />
              <KeyValueNetworkingPanel
                id={keyValue.id}
                isPublic={keyValue.public}
              />
              <KeyValuePlanSection
                keyValue={keyValue}
                onChanged={() => void refetch()}
              />
              <KeyValueMaxmemoryPolicySection id={keyValue.id} />
              <DatastoreMetricsPanel kind="keyvalue" resource={keyValue.name} />
              <KeyValueDangerActions
                keyValue={keyValue}
                onDeleted={() => void navigate({ to: "/" })}
                onChanged={() => void refetch()}
              />
            </>
          ) : (
            <>
              <MetadataListSkeleton rows={8} />
              <CardSkeleton rows={3} />
              <CardSkeleton rows={2} />
              <CardSkeleton rows={4} />
            </>
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}

function MetadataCard({
  keyValue,
  onRenamed,
}: {
  keyValue: KeyValueView;
  onRenamed: () => void;
}) {
  const { t } = useTranslations();
  return (
    <MetadataList
      title={t("keyvalue.metaTitle")}
      lead={<KeyValueNameRow keyValue={keyValue} onRenamed={onRenamed} />}
      rows={[
        {
          label: t("keyvalue.metaId"),
          value: (
            <code className="font-mono text-xs break-all">{keyValue.id}</code>
          ),
        },
        { label: t("keyvalue.metaStatus"), value: keyValue.status || "—" },
        { label: t("keyvalue.metaPlan"), value: keyValue.plan ?? "—" },
        {
          label: t("keyvalue.metaVersion"),
          value: keyValue.version ? `Valkey ${keyValue.version}` : "—",
        },
        {
          label: t("keyvalue.metaPublic"),
          value: keyValue.public ? t("keyvalue.yes") : t("keyvalue.no"),
        },
        {
          label: t("keyvalue.metaExternalHost"),
          value: keyValue.externalHost ?? "—",
        },
        ...(keyValue.region
          ? [
              {
                label: t("keyvalue.metaRegion"),
                value: keyValue.region,
              },
            ]
          : []),
        {
          label: t("keyvalue.metaCreated"),
          value: formatRelativeAge(keyValue.createdAt),
        },
      ]}
    />
  );
}

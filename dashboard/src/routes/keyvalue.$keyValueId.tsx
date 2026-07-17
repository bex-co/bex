import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import { MetadataList } from "@/common/components/metadata-list";
import { Skeleton } from "@/common/components/ui/skeleton";
import { cn } from "@/common/lib/utils/utils.ts";
import { formatRelativeAge } from "@/features/services/lib/format";
import { useKeyValue } from "@/features/keyvalue/hooks/use-key-value";
import { KeyValueStatusBadge } from "@/features/keyvalue/components/key-value-status-badge";
import { KeyValueRowActions } from "@/features/keyvalue/components/key-value-row-actions";
import { KeyValueLifecycleActions } from "@/features/keyvalue/components/key-value-lifecycle-actions";
import { ConnectionInfoPanel } from "@/features/keyvalue/components/connection-info-panel";
import { KeyValueNetworkingPanel } from "@/features/keyvalue/components/key-value-networking-panel";
import { KeyValuePlanSection } from "@/features/keyvalue/components/key-value-plan-section";
import { KeyValueNameSection } from "@/features/keyvalue/components/key-value-name-section";
import { KeyValueLogViewer } from "@/features/keyvalue/components/key-value-log-viewer";
import { DatastoreMetricsPanel } from "@/features/metrics/components/datastore-metrics-panel";
import type { KeyValueView } from "@/features/keyvalue/types";

export const Route = createFileRoute("/keyvalue/$keyValueId")({
  component: KeyValueDetailPage,
  beforeLoad: requireAuth("/keyvalue/$keyValueId"),
  validateSearch: (search: Record<string, unknown>): { tab?: "logs" } =>
    search.tab === "logs" ? { tab: "logs" } : {},
  head: ({ params }) => ({
    meta: [{ title: `${params.keyValueId} · Key Value · bex dashboard` }],
  }),
});

export function KeyValueDetailPage() {
  const { keyValueId } = Route.useParams();
  const { tab } = Route.useSearch();
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { keyValue, loading, refetch } = useKeyValue(keyValueId);

  const showNotFound = !loading && !keyValue;

  return (
    <DashboardLayout>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-4 sm:px-6">
        <div className="flex items-center gap-2">
          <h1
            className={cn(
              "truncate text-xl font-semibold",
              !keyValue && "text-muted-foreground",
            )}
          >
            {keyValue?.name ?? keyValueId}
          </h1>
          {keyValue ? <KeyValueStatusBadge status={keyValue.status} /> : null}
        </div>
        {keyValue ? (
          <KeyValueRowActions
            keyValue={keyValue}
            onDeleted={() => void navigate({ to: "/" })}
          />
        ) : null}
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
          {showNotFound ? (
            <div className="py-10 text-center">
              <p className="font-medium">{t("keyvalue.notFoundTitle")}</p>
              <p className="text-sm text-muted-foreground">
                {t("keyvalue.notFoundBody", { name: keyValueId })}
              </p>
            </div>
          ) : keyValue && tab === "logs" ? (
            <KeyValueLogViewer resource={keyValue.id} />
          ) : keyValue ? (
            <>
              <MetadataCard keyValue={keyValue} />
              <KeyValueNameSection
                key={`name-${keyValue.name}`}
                keyValue={keyValue}
                onChanged={() => void refetch()}
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
              <DatastoreMetricsPanel kind="keyvalue" resource={keyValue.name} />
              <KeyValueLifecycleActions
                keyValue={keyValue}
                onChanged={() => void refetch()}
              />
            </>
          ) : (
            <Skeleton className="h-64 w-full" />
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}

function MetadataCard({ keyValue }: { keyValue: KeyValueView }) {
  const { t } = useTranslations();
  return (
    <MetadataList
      title={t("keyvalue.metaTitle")}
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

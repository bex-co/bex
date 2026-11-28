import { useTranslations } from "@/common/hooks/use-translations";
import { MetadataList } from "@/common/components/metadata-list";
import { regionLabel } from "@/common/lib/region-label";
import { StatusWithReason } from "@/common/components/status-with-reason";
import { RelativeAge } from "@/common/components/relative-time";
import { DatabaseExpiryValue } from "@/features/databases/components/database-expiry";
import { DatabaseNameRow } from "@/features/databases/components/database-name-row";
import { DatabaseReadReplicas } from "@/features/databases/components/database-read-replicas";
import { DatabaseVersionValue } from "@/features/databases/components/database-upgrade-notices";
import {
  FREE_EXPIRED_REASON,
  statusLabel,
  statusReasonLabel,
} from "@/features/databases/lib/labels";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import { deriveStatus } from "@/features/databases/lib/status";
import type { DatabaseDetailView } from "@/features/databases/types";

/**
 * The detail page's Details card. It lives beside the database's other
 * building blocks (name row, version value, status badge, panels) rather
 * than inline in the route, so it can be rendered on its own in a test —
 * mirrors keyvalue's KeyValueMetadataCard. Its last field is the read
 * replicas, as Render's General card ends its facts with Read Replicas
 * (w3/116).
 */
export function DatabaseMetadataCard({
  database,
  onRenamed,
}: {
  database: DatabaseDetailView;
  onRenamed: () => void;
}) {
  const { t } = useTranslations();
  const { instanceTypes } = useDatabaseInstanceTypes();
  // Cache-first catalog: fall back to the plan id while it is empty.
  const planName =
    instanceTypes.find((it) => it.id === database.plan)?.name ?? database.plan;
  // Only beside "Unavailable": the list query refreshes the cached status
  // without the reason, so a recovered database could still hold a stale one.
  // The shrink refusal names the allocated size, which diskSizeGB reports.
  const reasonKey = statusReasonLabel(database.statusReasonCode);
  // An expired free database is suspended with its own reason (w8/m126).
  const statusKey = deriveStatus(database).key;
  const reason =
    statusKey === "unavailable" ||
    (statusKey === "suspended" &&
      database.statusReasonCode === FREE_EXPIRED_REASON)
      ? reasonKey
        ? t(reasonKey, { size: database.diskSizeGB ?? "" })
        : database.statusReason
      : null;
  return (
    <MetadataList
      title={t("databases.metaTitle")}
      lead={<DatabaseNameRow database={database} onRenamed={onRenamed} />}
      footer={<DatabaseReadReplicas database={database} />}
      rows={[
        // The same label the header badge shows: a suspended instance still
        // reports status "available", so the raw value contradicted the badge
        // beside it (w1/m159, from w1/085).
        {
          label: t("databases.metaStatus"),
          value: (
            <StatusWithReason
              status={t(statusLabel(database))}
              reason={reason}
            />
          ),
        },
        { label: t("databases.metaPlan"), value: planName ?? "—" },
        {
          label: t("databases.metaVersion"),
          // Render's version value links its guided upgrade page (w3/m676).
          value: <DatabaseVersionValue database={database} />,
        },
        {
          label: t("databases.metaDatabaseName"),
          value: database.databaseName ?? "—",
        },
        {
          label: t("databases.metaDatabaseUser"),
          // The user to connect as: the current default login, which a
          // credential rotation moves off the create-time owner (w4/610).
          value: database.defaultUser ?? database.databaseUser ?? "—",
        },
        {
          label: t("databases.metaStorage"),
          value: database.diskSizeGB ? `${database.diskSizeGB} GB` : "—",
        },
        {
          label: t("databases.metaHighAvailability"),
          value: database.highAvailabilityEnabled
            ? t("databases.yes")
            : t("databases.no"),
        },
        {
          label: t("databases.metaPublic"),
          value: database.public ? t("databases.yes") : t("databases.no"),
        },
        // Render shows external connection details only when public access is
        // on; the backend sends "" (not null) for a private database (w6/052),
        // so gate on truthiness and omit the row entirely — the region-row
        // pattern below.
        ...(database.externalHost
          ? [
              {
                label: t("databases.metaExternalHost"),
                value: database.externalHost,
              },
            ]
          : []),
        ...(database.region
          ? [
              {
                label: t("databases.metaRegion"),
                value: regionLabel(database.region, t),
              },
            ]
          : []),
        {
          label: t("databases.metaCreated"),
          value: <RelativeAge value={database.createdAt} />,
        },
        // Free databases only: paid plans never expire (w8/m126).
        ...(database.expiresAt
          ? [
              {
                label: t("databases.metaExpires"),
                value: (
                  <DatabaseExpiryValue
                    databaseId={database.id}
                    expiresAt={database.expiresAt}
                  />
                ),
              },
            ]
          : []),
      ]}
    />
  );
}

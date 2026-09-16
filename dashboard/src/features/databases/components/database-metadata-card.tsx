import { useTranslations } from "@/common/hooks/use-translations";
import { MetadataList } from "@/common/components/metadata-list";
import { RelativeAge } from "@/common/components/relative-time";
import { DatabaseNameRow } from "@/features/databases/components/database-name-row";
import { DatabaseVersionControl } from "@/features/databases/components/database-version-control";
import { statusLabel } from "@/features/databases/lib/labels";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import type { DatabaseDetailView } from "@/features/databases/types";

/**
 * The detail page's Details card. It lives beside the database's other
 * building blocks (name row, version control, status badge, panels) rather
 * than inline in the route, so it can be rendered on its own in a test —
 * mirrors keyvalue's KeyValueMetadataCard.
 */
export function DatabaseMetadataCard({
  database,
  onVersionChanged,
  onRenamed,
}: {
  database: DatabaseDetailView;
  onVersionChanged: () => void;
  onRenamed: () => void;
}) {
  const { t } = useTranslations();
  const { instanceTypes } = useDatabaseInstanceTypes();
  // Cache-first catalog: fall back to the plan id while it is empty.
  const planName =
    instanceTypes.find((it) => it.id === database.plan)?.name ?? database.plan;
  return (
    <MetadataList
      title={t("databases.metaTitle")}
      lead={<DatabaseNameRow database={database} onRenamed={onRenamed} />}
      rows={[
        // The same label the header badge shows: a suspended instance still
        // reports status "available", so the raw value contradicted the badge
        // beside it (w1/m159, from w1/085).
        { label: t("databases.metaStatus"), value: t(statusLabel(database)) },
        { label: t("databases.metaPlan"), value: planName ?? "—" },
        {
          label: t("databases.metaVersion"),
          value: (
            <DatabaseVersionControl
              database={database}
              onChanged={onVersionChanged}
            />
          ),
        },
        {
          label: t("databases.metaDatabaseName"),
          value: database.databaseName ?? "—",
        },
        {
          label: t("databases.metaDatabaseUser"),
          value: database.databaseUser ?? "—",
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
                value: database.region,
              },
            ]
          : []),
        {
          label: t("databases.metaCreated"),
          value: <RelativeAge value={database.createdAt} />,
        },
      ]}
    />
  );
}

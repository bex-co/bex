import { useTranslations } from "@/common/hooks/use-translations";
import { MetadataList } from "@/common/components/metadata-list";
import { RelativeAge } from "@/common/components/relative-time";
import { KeyValueNameRow } from "@/features/keyvalue/components/key-value-name-row";
import { statusLabel } from "@/features/keyvalue/lib/labels";
import { useKeyValueInstanceTypes } from "@/features/keyvalue/hooks/use-key-value-instance-types";
import type { KeyValueView } from "@/features/keyvalue/types";

/**
 * The detail page's Details card. It lives beside the store's other building
 * blocks (name row, status badge, panels) rather than inline in the route, so
 * it can be rendered on its own in a test — mirrors databases'
 * DatabaseMetadataCard.
 */
export function KeyValueMetadataCard({
  keyValue,
  onRenamed,
}: {
  keyValue: KeyValueView;
  onRenamed: () => void;
}) {
  const { t } = useTranslations();
  const { instanceTypes } = useKeyValueInstanceTypes();
  // The catalog is cache-first and may be empty on first paint; fall back to
  // the plan id rather than showing nothing.
  const planName =
    instanceTypes.find((it) => it.id === keyValue.plan)?.name ?? keyValue.plan;
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
        // The same label the header badge shows: a suspended store still
        // reports status "available", so printing the wire value made this row
        // contradict the badge beside it (w1/m159, from w1/085).
        { label: t("keyvalue.metaStatus"), value: t(statusLabel(keyValue)) },
        { label: t("keyvalue.metaPlan"), value: planName ?? "—" },
        {
          label: t("keyvalue.metaVersion"),
          value: keyValue.version ? `Valkey ${keyValue.version}` : "—",
        },
        {
          label: t("keyvalue.metaPublic"),
          value: keyValue.public ? t("keyvalue.yes") : t("keyvalue.no"),
        },
        // Render shows external connection details only when public access is
        // on; the backend sends "" (not null) for a private store (w6/052), so
        // gate on truthiness and omit the row entirely — the region-row
        // pattern below.
        ...(keyValue.externalHost
          ? [
              {
                label: t("keyvalue.metaExternalHost"),
                value: keyValue.externalHost,
              },
            ]
          : []),
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
          value: <RelativeAge value={keyValue.createdAt} />,
        },
      ]}
    />
  );
}

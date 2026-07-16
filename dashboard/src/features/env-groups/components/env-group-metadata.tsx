import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import type { EnvGroupView } from "@/features/env-groups/types";

export function EnvGroupMetadata({
  group,
  className,
}: {
  group: EnvGroupView;
  className?: string;
}) {
  const { t } = useTranslations();
  return (
    <dl className={cn("grid gap-3 text-sm sm:grid-cols-3", className)}>
      <MetadataItem label={t("envGroups.ownerLabel")} value={group.ownerId} />
      <MetadataItem
        label={t("envGroups.createdAtLabel")}
        value={group.createdAt}
        timestamp
      />
      <MetadataItem
        label={t("envGroups.updatedAtLabel")}
        value={group.updatedAt}
        timestamp
      />
    </dl>
  );
}

function MetadataItem({
  label,
  value,
  timestamp = false,
}: {
  label: string;
  value: string | null;
  timestamp?: boolean;
}) {
  return (
    <div className="min-w-0">
      <dt className="text-xs font-medium text-muted-foreground">{label}</dt>
      <dd className="truncate font-mono text-xs" title={value ?? undefined}>
        {timestamp && value ? (
          <time dateTime={value}>{value}</time>
        ) : (
          (value ?? "—")
        )}
      </dd>
    </div>
  );
}

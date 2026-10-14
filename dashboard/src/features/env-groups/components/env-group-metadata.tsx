import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import { LocalDateTime } from "@/common/components/local-time";
import type { EnvGroupView } from "@/features/env-groups/types";

/** An Environment lookup that has not produced a name: still loading, or the
 *  scope read failed. Neither may be rendered as "Unknown Environment" — that
 *  is reserved for a lookup that succeeded without the id (w4/304). */
export type EnvironmentLookupStatus = "loading" | "error";

export function EnvGroupMetadata({
  group,
  environmentLabel,
  environmentStatus,
  className,
}: {
  group: EnvGroupView;
  environmentLabel?: string;
  /** When set, replaces `environmentLabel` in the Environment value slot. */
  environmentStatus?: EnvironmentLookupStatus;
  className?: string;
}) {
  const { t } = useTranslations();
  return (
    <dl className={cn("grid gap-3 text-sm sm:grid-cols-4", className)}>
      <MetadataItem label={t("envGroups.ownerLabel")} value={group.ownerId} />
      <MetadataItem
        label={t("envGroups.environmentLabel")}
        value={
          environmentStatus === "loading"
            ? t("envGroups.environmentLookupLoading")
            : environmentStatus === "error"
              ? t("envGroups.environmentLookupFailed")
              : (environmentLabel ?? null)
        }
        status={environmentStatus}
      />
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
      {group.availability ? (
        <MetadataItem
          label={t("envGroups.availabilityLabel")}
          value={
            group.availability === "busy"
              ? t("envGroups.availabilityBusy")
              : t("envGroups.availabilityRepairRequired")
          }
        />
      ) : null}
    </dl>
  );
}

function MetadataItem({
  label,
  value,
  timestamp = false,
  status,
}: {
  label: string;
  value: string | null;
  timestamp?: boolean;
  status?: EnvironmentLookupStatus;
}) {
  return (
    <div className="min-w-0">
      <dt className="text-xs font-medium text-muted-foreground">{label}</dt>
      {/* A status value keeps the same single-line truncating slot; only its
          tone changes, and the pending one is announced as a live status. */}
      <dd
        className={cn(
          "truncate font-mono text-xs",
          status && "text-muted-foreground",
        )}
        title={value ?? undefined}
        data-lookup-status={status}
      >
        {status === "loading" ? (
          <span role="status">{value}</span>
        ) : timestamp && value ? (
          <LocalDateTime value={value} fallback={value} />
        ) : (
          (value ?? "—")
        )}
      </dd>
    </div>
  );
}

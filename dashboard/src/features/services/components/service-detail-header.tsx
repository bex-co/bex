import { Skeleton } from "@/common/components/ui/skeleton.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import { ServiceRowActions } from "@/features/services/components/service-row-actions";
import { ServiceStatusBadge } from "@/features/services/components/service-status-badge";
import { isSleeping } from "@/features/services/lib/status";
import type { ServiceView, LifecycleAction } from "@/features/services/types";

export interface ServiceDetailHeaderProps {
  service: ServiceView;
  /** The lifecycle action in flight for this service, or null. */
  pending: LifecycleAction | null;
  onRun: (action: LifecycleAction, service: ServiceView) => void;
}

/**
 * The service-detail header — name, status badge, live URL, and the lifecycle
 * actions — mirroring Render's service-page header. Reuses the list's
 * `ServiceRowActions` (and, via it, m4's confirm + poll-to-converge) so the
 * verbs behave identically on the list and the detail page.
 */
export function ServiceDetailHeader({
  service,
  pending,
  onRun,
}: ServiceDetailHeaderProps) {
  const { t } = useTranslations();
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-4 sm:px-6">
      <div className="min-w-0 space-y-1">
        <div className="flex items-center gap-2">
          <h1 className="truncate text-xl font-semibold">{service.name}</h1>
          <ServiceStatusBadge service={service} />
        </div>
        {service.url ? (
          <a
            href={service.url}
            target="_blank"
            rel="noreferrer"
            className="text-sm text-muted-foreground hover:underline"
          >
            {service.url}
          </a>
        ) : null}
        {isSleeping(service) ? (
          <p className="text-sm text-muted-foreground">
            {t("services.statusSleepingHint")}
          </p>
        ) : null}
      </div>
      <ServiceRowActions service={service} pending={pending} onRun={onRun} />
    </div>
  );
}

/** Placeholder header shown while `server(id)` is still loading. */
export function ServiceDetailHeaderSkeleton({ name }: { name: string }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b px-4 py-4 sm:px-6">
      <div className="space-y-2">
        <h1 className="truncate text-xl font-semibold text-muted-foreground">
          {name}
        </h1>
        <Skeleton className="h-4 w-48" />
      </div>
      <Skeleton className="size-8 rounded-md" />
    </div>
  );
}

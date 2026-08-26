import { Link } from "@tanstack/react-router";
import { AlertCircle, CircleDot } from "lucide-react";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { useServiceBase } from "@/features/services/lib/service-base";
import { RelativeAge } from "@/common/components/relative-time";
import { useServiceEvents } from "@/features/events/hooks/use-service-events";
import {
  filterTimelineEvents,
  type EventTimelineFilter,
} from "@/features/events/lib/timeline";

// The category filter lives in the Metrics toolbar (Render's filter-events
// control, w5/m42) — the timeline itself just renders whatever the filter
// admits, so it stays a pure presentation of the selected range.
export function EventTimeline({
  serviceId,
  startTime,
  endTime,
  filter = "all",
}: {
  serviceId: string;
  startTime: string;
  endTime: string;
  filter?: EventTimelineFilter;
}) {
  const { t } = useTranslations();
  const base = useServiceBase();
  const { events, loading, error } = useServiceEvents(serviceId, {
    limit: 100,
    startTime,
    endTime,
    autoPaginate: true,
  });
  const visible = filterTimelineEvents(events, startTime, endTime, filter);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("metrics.eventsTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading && events.length === 0 ? (
          <div
            className="space-y-2"
            role="status"
            aria-label={t("metrics.eventsLoading")}
          >
            {[0, 1, 2].map((i) => (
              <div key={i} className="flex items-center gap-2">
                <Skeleton className="size-4 shrink-0 rounded-full" />
                <Skeleton className="h-3 w-40" />
              </div>
            ))}
          </div>
        ) : error ? (
          <div className="flex min-h-20 items-center justify-center gap-2 text-sm text-muted-foreground">
            <AlertCircle className="size-4" />
            {t("metrics.eventsError")}
          </div>
        ) : visible.length === 0 ? (
          <div className="min-h-20 py-4 text-center" role="status">
            <p className="text-sm font-medium">
              {t("metrics.eventsEmptyTitle")}
            </p>
            <p className="mt-1 text-xs text-muted-foreground">
              {t("metrics.eventsEmptyBody")}
            </p>
          </div>
        ) : (
          <ol
            className="flex gap-3 overflow-x-auto pb-1"
            aria-label={t("metrics.eventsTitle")}
          >
            {visible.map((event) => {
              const deployId = event.details?.deployId ?? "";
              const item = (
                <div className="min-w-40 rounded-md border p-3">
                  <div className="flex items-center gap-2">
                    <CircleDot className="size-3.5 text-primary" />
                    <span className="truncate text-sm font-medium">
                      {event.type?.replaceAll("_", " ") ||
                        t("metrics.eventsChanged")}
                    </span>
                  </div>
                  {event.timestamp ? (
                    <RelativeAge
                      value={event.timestamp}
                      className="mt-1 block text-xs text-muted-foreground"
                    />
                  ) : null}
                </div>
              );
              return (
                <li key={event.id}>
                  {deployId ? (
                    <Link
                      to={`${base}/$serviceId/deploys/$deployId`}
                      params={{ serviceId, deployId }}
                      className="block rounded-md hover:bg-muted/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    >
                      {item}
                    </Link>
                  ) : (
                    item
                  )}
                </li>
              );
            })}
          </ol>
        )}
      </CardContent>
    </Card>
  );
}

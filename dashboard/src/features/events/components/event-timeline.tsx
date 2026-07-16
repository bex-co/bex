import { Link } from "@tanstack/react-router";
import { AlertCircle, CircleDot, Loader2 } from "lucide-react";
import { useState } from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatRelativeAge } from "@/features/services/lib/format";
import { useServiceEvents } from "@/features/events/hooks/use-service-events";
import {
  filterTimelineEvents,
  type EventTimelineFilter,
} from "@/features/events/lib/timeline";

export function EventTimeline({
  serviceId,
  startTime,
  endTime,
}: {
  serviceId: string;
  startTime: string;
  endTime: string;
}) {
  const { t } = useTranslations();
  const { events, loading, error } = useServiceEvents(serviceId, 100);
  const [filter, setFilter] = useState<EventTimelineFilter>("all");
  const visible = filterTimelineEvents(events, startTime, endTime, filter);

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between">
        <CardTitle>{t("metrics.eventsTitle")}</CardTitle>
        <Select
          value={filter}
          onValueChange={(value) => setFilter(value as EventTimelineFilter)}
        >
          <SelectTrigger
            size="sm"
            className="w-40"
            aria-label={t("metrics.eventsFilterLabel")}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("metrics.eventsFilterAll")}</SelectItem>
            <SelectItem value="deploy">
              {t("metrics.eventsFilterDeploys")}
            </SelectItem>
            <SelectItem value="lifecycle">
              {t("metrics.eventsFilterLifecycle")}
            </SelectItem>
            <SelectItem value="config">
              {t("metrics.eventsFilterConfig")}
            </SelectItem>
          </SelectContent>
        </Select>
      </CardHeader>
      <CardContent>
        {loading && events.length === 0 ? (
          <div className="flex min-h-20 items-center justify-center text-sm text-muted-foreground">
            <Loader2 className="mr-2 size-4 animate-spin" />
            {t("metrics.eventsLoading")}
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
                    <time
                      className="mt-1 block text-xs text-muted-foreground"
                      dateTime={event.timestamp}
                    >
                      {formatRelativeAge(event.timestamp)}
                    </time>
                  ) : null}
                </div>
              );
              return (
                <li key={event.id}>
                  {deployId ? (
                    <Link
                      to="/services/$serviceId/deploys/$deployId"
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

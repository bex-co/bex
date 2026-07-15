import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@apollo/client/react";
import { ServiceEventsDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "@/common/components/ui/card";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useServer } from "@/features/services/hooks/use-server";
import { CronRunsSection } from "@/features/services/components/cron-runs-section";
import { isCron } from "@/features/services/lib/service-type";
import {
  deployStatusVariant as statusVariant,
  deployStatusKey as statusKey,
  preDeployStatusKey as preDeployKey,
  isCancelableDeployStatus,
} from "@/features/deploys/lib/deploy-status";
import { DeployActions } from "@/features/deploys/components/deploy-actions";

export const Route = createFileRoute("/services/$serviceId/events")({
  component: ServiceEventsPage,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Events · bex dashboard` }],
  }),
});

// ServiceEventDetails.trigger is the Trigger object (boolean flags), a
// different shape from Deploy.trigger's plain string — derive a short label
// from whichever flag is set, first-match-wins.
type TriggerFlags = {
  firstBuild?: boolean | null;
  envUpdated?: boolean | null;
  manual?: boolean | null;
  deployedByRender?: boolean | null;
  clearCache?: boolean | null;
  rollback?: boolean | null;
} | null;

function triggerLabel(trigger: TriggerFlags): string | null {
  if (!trigger) return null;
  if (trigger.rollback) return "rollback";
  if (trigger.firstBuild) return "first build";
  if (trigger.manual) return "manual";
  if (trigger.envUpdated) return "env updated";
  if (trigger.clearCache) return "clear cache";
  if (trigger.deployedByRender) return "deployed by render";
  return null;
}

export function ServiceEventsPage() {
  const { serviceId } = Route.useParams();
  const { t } = useTranslations();
  const { data, loading, refetch } = useQuery(ServiceEventsDocument, {
    variables: { serviceId, limit: 20 },
    fetchPolicy: "cache-and-network",
  });

  // A cron_job's first-class run history hangs off the same landing tab.
  const { service } = useServer(serviceId);

  const events = (data?.serviceEvents ?? []).filter(
    (event): event is NonNullable<typeof event> & { id: string } =>
      event != null && !!event.id,
  );

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>{t("services.eventsTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          {loading && events.length === 0 ? (
            <div className="space-y-3">
              {[0, 1, 2].map((i) => (
                <Skeleton key={i} className="h-14 w-full" />
              ))}
            </div>
          ) : events.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("services.eventsEmpty")}
            </p>
          ) : (
            <div className="divide-y">
              {events.map((event) => {
                const details = event.details;
                const deployId = details?.deployId ?? "";
                // Render's deploy_started event intentionally has no terminal
                // status, while deploy_ended uses succeeded/failed instead of
                // the deploy object's live/update_failed vocabulary. Normalize
                // that API boundary for the shared badge and action helpers.
                const status =
                  event.type === "deploy_started"
                    ? "update_in_progress"
                    : details?.deployStatus === "succeeded"
                      ? "live"
                      : details?.deployStatus === "failed"
                        ? "update_failed"
                        : (details?.deployStatus ?? "");
                const label = triggerLabel(details?.trigger ?? null);
                const preDeploy = preDeployKey(details?.preDeployStatus ?? "");
                const summary = (
                  <EventSummary
                    status={status}
                    label={label}
                    preDeployStatus={details?.preDeployStatus ?? ""}
                    preDeploy={preDeploy}
                    timestamp={event.timestamp}
                  />
                );
                const hasAction =
                  !!deployId &&
                  (isCancelableDeployStatus(status) || status === "live");

                return (
                  <div
                    key={event.id}
                    className="flex items-start justify-between gap-4 py-3"
                  >
                    {deployId ? (
                      <Link
                        to="/services/$serviceId/deploys/$deployId"
                        params={{ serviceId, deployId }}
                        className="min-w-0 flex-1 rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        {summary}
                      </Link>
                    ) : (
                      <div className="min-w-0 flex-1">{summary}</div>
                    )}
                    {hasAction ? (
                      <DeployActions
                        serviceId={serviceId}
                        deployId={deployId}
                        status={status}
                        onChanged={() => void refetch()}
                      />
                    ) : null}
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      {service && isCron(service) ? (
        <CronRunsSection serviceId={serviceId} />
      ) : null}
    </div>
  );
}

function EventSummary({
  status,
  label,
  preDeployStatus,
  preDeploy,
  timestamp,
}: {
  status: string;
  label: string | null;
  preDeployStatus: string;
  preDeploy: string | null;
  timestamp: string | null;
}) {
  const { t } = useTranslations();
  return (
    <>
      <div className="flex items-center gap-2">
        <Badge variant={statusVariant(status)}>
          {t(statusKey(status) as Parameters<typeof t>[0])}
        </Badge>
        {label ? (
          <span className="text-xs capitalize text-muted-foreground">
            {label}
          </span>
        ) : null}
      </div>
      {preDeploy ? (
        <p
          className={`mt-1 text-xs ${
            preDeployStatus === "failed"
              ? "text-destructive"
              : "text-muted-foreground"
          }`}
        >
          {t(preDeploy as Parameters<typeof t>[0])}
        </p>
      ) : null}
      {timestamp ? (
        <p className="mt-1 text-xs text-muted-foreground">
          {new Date(timestamp).toLocaleString()}
        </p>
      ) : null}
    </>
  );
}

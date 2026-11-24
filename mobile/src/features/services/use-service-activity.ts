import { useEffect, useMemo, useState } from "react";
import { NetworkStatus } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import { refetchUnlessSkipped } from "@/common/apollo/refetch-unless-skipped";
import {
  MobileDeployHistoryDocument,
  MobileServiceEventsDocument,
  type MobileDeployHistoryQuery,
  type MobileServiceEventsQuery,
} from "@/generated-graphql";
import {
  appendUnique,
  mergeTimeline,
  type TimelineDeploy,
  type TimelineEvent,
} from "@/features/events/timeline";
import type { MobileDeployActionTarget } from "./service-actions-card";

const PAGE_SIZE = 20;
type DeployNode = NonNullable<
  NonNullable<MobileDeployHistoryQuery["deploys"]>[number]
>;
type EventNode = NonNullable<
  NonNullable<MobileServiceEventsQuery["serviceEvents"]>[number]
>;

// A service's deploy history and events, paged together into one timeline.
// Disabled (no can_view_activity: Render's "View service events", w9/m621),
// both reads are skipped and the timeline is empty, idle and error-free.
// Apollo's standby emits nothing, so a skipped query can keep its last data
// or error; the gating below is what keeps them off the screen.
export function useServiceActivity({
  serviceId,
  enabled,
  pollInterval,
}: {
  serviceId: string;
  enabled: boolean;
  pollInterval: number;
}) {
  const options = {
    variables: { serviceId, limit: PAGE_SIZE },
    skip: !enabled,
    fetchPolicy: "cache-and-network" as const,
    errorPolicy: "all" as const,
    notifyOnNetworkStatusChange: true,
    pollInterval,
  };
  const deployQuery = useQuery(MobileDeployHistoryDocument, options);
  const eventQuery = useQuery(MobileServiceEventsDocument, options);
  const [extraDeploys, setExtraDeploys] = useState<DeployNode[]>([]);
  const [extraEvents, setExtraEvents] = useState<EventNode[]>([]);
  const [hasMoreDeploys, setHasMoreDeploys] = useState(true);
  const [hasMoreEvents, setHasMoreEvents] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);

  useEffect(() => {
    setExtraDeploys([]);
    setExtraEvents([]);
    setHasMoreDeploys(true);
    setHasMoreEvents(true);
  }, [serviceId]);

  const deploys = useMemo(
    () =>
      enabled
        ? appendUnique(
            (deployQuery.data?.deploys ?? []).filter(
              (deploy): deploy is DeployNode => Boolean(deploy),
            ),
            extraDeploys,
            (deploy) => deploy.id ?? null,
          )
        : [],
    [deployQuery.data, enabled, extraDeploys],
  );
  const events = useMemo(
    () =>
      enabled
        ? appendUnique(
            (eventQuery.data?.serviceEvents ?? []).filter(
              (event): event is EventNode => Boolean(event),
            ),
            extraEvents,
            (event) => event.id ?? null,
          )
        : [],
    [enabled, eventQuery.data, extraEvents],
  );
  const timeline = useMemo(
    () => mergeTimeline(deploys as TimelineDeploy[], events as TimelineEvent[]),
    [deploys, events],
  );
  const deployTargets = useMemo(() => actionTargets(deploys), [deploys]);

  async function loadMore() {
    if (loadingMore || !enabled) return;
    setLoadingMore(true);
    try {
      const requests: Promise<void>[] = [];
      if (hasMoreDeploys && deploys.at(-1)?.id) {
        requests.push(
          deployQuery
            .fetchMore({
              variables: { cursor: deploys.at(-1)?.id, limit: PAGE_SIZE },
            })
            .then(({ data }) => {
              const page = (data?.deploys ?? []).filter(
                (deploy): deploy is DeployNode => Boolean(deploy),
              );
              setExtraDeploys((current) =>
                appendUnique(current, page, (deploy) => deploy.id ?? null),
              );
              setHasMoreDeploys(page.length === PAGE_SIZE);
            }),
        );
      } else {
        setHasMoreDeploys(false);
      }
      if (hasMoreEvents && events.at(-1)?.cursor) {
        requests.push(
          eventQuery
            .fetchMore({
              variables: { cursor: events.at(-1)?.cursor, limit: PAGE_SIZE },
            })
            .then(({ data }) => {
              const page = (data?.serviceEvents ?? []).filter(
                (event): event is EventNode => Boolean(event),
              );
              setExtraEvents((current) =>
                appendUnique(current, page, (event) => event.id ?? null),
              );
              setHasMoreEvents(page.length === PAGE_SIZE);
            }),
        );
      } else {
        setHasMoreEvents(false);
      }
      await Promise.all(requests);
    } finally {
      setLoadingMore(false);
    }
  }

  return {
    timeline,
    deployTargets,
    loading: enabled && (deployQuery.loading || eventQuery.loading),
    error: enabled ? deployQuery.error || eventQuery.error : undefined,
    refreshing:
      enabled &&
      (deployQuery.networkStatus === NetworkStatus.refetch ||
        eventQuery.networkStatus === NetworkStatus.refetch),
    hasMore: hasMoreDeploys || hasMoreEvents,
    loadingMore,
    loadMore,
    refetch: () =>
      Promise.all([
        refetchUnlessSkipped(!enabled, () => deployQuery.refetch()),
        refetchUnlessSkipped(!enabled, () => eventQuery.refetch()),
      ]),
    // The first page, re-read for the deploy actions' confirm-time targets.
    refetchDeploys: async () => {
      const result = await refetchUnlessSkipped(!enabled, () =>
        deployQuery.refetch(),
      );
      return actionTargets(result?.data?.deploys ?? []);
    },
  };
}

function actionTargets(
  deploys: readonly (DeployNode | null)[],
): MobileDeployActionTarget[] {
  return deploys.flatMap((deploy) =>
    deploy?.id
      ? [
          {
            id: deploy.id,
            status: deploy.status ?? "unknown",
            rollbackOf: deploy.rollbackOf,
          },
        ]
      : [],
  );
}

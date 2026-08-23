import { useEffect, useMemo, useRef, useState } from "react";
import { NetworkStatus } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import { router } from "expo-router";
import { ActivityIndicator, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { Ionicons } from "@expo/vector-icons";
import { DashboardCard } from "@/components/dashboard-card";
import { DashboardScrollView } from "@/components/dashboard-scroll-view";
import { DetailHeader } from "@/components/detail-header";
import { Button } from "@/components/button";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useRecovery,
  useRecoveryEnvironment,
} from "@/common/hooks/use-recovery";
import { recoveryAvailable } from "@/common/hooks/recovery-coordinator";
import { formatTimestamp, humanizeToken } from "@/common/format-util";
import { fonts, fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import {
  MobileDeployHistoryDocument,
  MobileServiceEventsDocument,
  MobileServiceSupervisionDocument,
  type MobileDeployHistoryQuery,
  type MobileServiceEventsQuery,
  type MobileServiceSupervisionQuery,
} from "@/generated-graphql";
import { MetricSnapshots } from "@/features/metrics/metric-snapshots";
import {
  CronRunsCard,
  type CronRunsCardHandle,
} from "@/features/cron/cron-runs-card";
import {
  EnvironmentCard,
  type EnvironmentCardHandle,
} from "@/features/environment/environment-card";
import {
  appendUnique,
  knownEventType,
  mergeTimeline,
  type TimelineDeploy,
  type TimelineEvent,
  type TimelineItem,
} from "@/features/events/timeline";
import { statusToneColor } from "@/features/resources/resource-groups";
import { StatusBadge } from "@/features/resources/status-badge";
import { ServiceActionsCard } from "./service-actions-card";
import type { ServiceLifecycleResource } from "./lifecycle";

const PAGE_SIZE = 20;
type DeployNode = NonNullable<
  NonNullable<MobileDeployHistoryQuery["deploys"]>[number]
>;
type EventNode = NonNullable<
  NonNullable<MobileServiceEventsQuery["serviceEvents"]>[number]
>;

export function ServiceDetailScreen({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const recoveryEnvironment = useRecoveryEnvironment();
  const environmentRef = useRef<EnvironmentCardHandle>(null);
  const cronRunsRef = useRef<CronRunsCardHandle>(null);
  const pollInterval = recoveryAvailable(recoveryEnvironment) ? 30_000 : 0;
  const serviceQuery = useQuery(MobileServiceSupervisionDocument, {
    variables: { id: serviceId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    notifyOnNetworkStatusChange: true,
    pollInterval,
  });
  const deployQuery = useQuery(MobileDeployHistoryDocument, {
    variables: { serviceId, limit: PAGE_SIZE },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    notifyOnNetworkStatusChange: true,
    pollInterval,
  });
  const eventQuery = useQuery(MobileServiceEventsDocument, {
    variables: { serviceId, limit: PAGE_SIZE },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    notifyOnNetworkStatusChange: true,
    pollInterval,
  });
  const recovery = useRecovery({
    attempt: async () => {
      await Promise.all([
        serviceQuery.refetch(),
        deployQuery.refetch(),
        eventQuery.refetch(),
        environmentRef.current?.refresh() ?? Promise.resolve(),
        cronRunsRef.current?.refresh() ?? Promise.resolve(),
      ]);
    },
  });
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
      appendUnique(
        (deployQuery.data?.deploys ?? []).filter(
          (deploy): deploy is DeployNode => Boolean(deploy),
        ),
        extraDeploys,
        (deploy) => deploy.id ?? null,
      ),
    [deployQuery.data, extraDeploys],
  );
  const events = useMemo(
    () =>
      appendUnique(
        (eventQuery.data?.serviceEvents ?? []).filter(
          (event): event is EventNode => Boolean(event),
        ),
        extraEvents,
        (event) => event.id ?? null,
      ),
    [eventQuery.data, extraEvents],
  );
  const timeline = useMemo(
    () => mergeTimeline(deploys as TimelineDeploy[], events as TimelineEvent[]),
    [deploys, events],
  );
  const service = serviceQuery.data?.service;
  const serviceName = service?.displayName || service?.name;
  const initialLoading =
    (serviceQuery.loading || deployQuery.loading || eventQuery.loading) &&
    !service &&
    timeline.length === 0;
  const anyError = serviceQuery.error || deployQuery.error || eventQuery.error;

  async function loadMore() {
    if (loadingMore) return;
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

  return (
    <SafeAreaView style={[styles.safe, { backgroundColor: theme.background }]}>
      <DashboardScrollView
        refreshing={
          serviceQuery.networkStatus === 4 ||
          deployQuery.networkStatus === 4 ||
          eventQuery.networkStatus === 4
        }
        onRefresh={() => void recovery.manualRetry()}
        contentContainerStyle={styles.content}
      >
        <DetailHeader
          title={serviceName || t("service.title")}
          subtitle={humanizeToken(
            service?.type || service?.runtime || t("resources.service"),
          )}
        />

        {anyError ? (
          <View
            accessibilityRole="alert"
            style={[styles.notice, { borderColor: theme.warning }]}
          >
            <Text style={{ color: theme.warning }}>
              {service || timeline.length
                ? t("service.partialError")
                : t("service.loadError")}
            </Text>
          </View>
        ) : null}

        {initialLoading ? (
          <DashboardCard>
            <ActivityIndicator
              color={theme.primary}
              style={styles.initialLoader}
            />
          </DashboardCard>
        ) : service ? (
          <>
            <ServiceIdentityCard service={service} />
            <EnvironmentCard
              ref={environmentRef}
              serviceId={serviceId}
              serviceLabel={serviceName || serviceId}
            />
            {service.id ? (
              <ServiceActionsCard
                service={serviceLifecycleResource(service)}
                deploys={deploys.flatMap((deploy) =>
                  deploy.id
                    ? [
                        {
                          id: deploy.id,
                          status: deploy.status ?? "unknown",
                          rollbackOf: deploy.rollbackOf,
                        },
                      ]
                    : [],
                )}
                refreshService={async () => {
                  const result = await serviceQuery.refetch();
                  const refreshed = result.data?.service;
                  return refreshed?.id
                    ? serviceLifecycleResource(refreshed)
                    : null;
                }}
                refreshDeploys={async () => {
                  const result = await deployQuery.refetch();
                  return (result.data?.deploys ?? []).flatMap((deploy) =>
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
                }}
              />
            ) : null}
            {service.id && service.type?.toLowerCase() === "cron_job" ? (
              <CronRunsCard
                key={service.id}
                ref={cronRunsRef}
                serviceId={service.id}
                serviceLabel={serviceName || service.id}
                suspended={service.suspended}
                serviceEvidenceCurrent={
                  serviceQuery.networkStatus === NetworkStatus.ready &&
                  !serviceQuery.error
                }
                onOpenLogs={() =>
                  router.push(
                    `/services/${encodeURIComponent(service.id!)}/logs`,
                  )
                }
              />
            ) : null}
          </>
        ) : null}

        <MetricSnapshots resourceId={serviceId} />

        <DashboardCard title={t("logs.title")}>
          <Text style={[styles.logBody, { color: theme.mutedForeground }]}>
            {t("logs.description")}
          </Text>
          <Button
            onPress={() =>
              router.push(`/services/${encodeURIComponent(serviceId)}/logs`)
            }
            accessibilityLabel={t("logs.open")}
          >
            {t("logs.open")}
          </Button>
        </DashboardCard>

        <DashboardCard title={t("activity.timelineTitle")} bleed>
          {timeline.length ? (
            timeline.map((item) => <TimelineRow key={item.key} item={item} />)
          ) : !initialLoading ? (
            <Text style={[styles.empty, { color: theme.mutedForeground }]}>
              {t("activity.empty")}
            </Text>
          ) : null}
          {(hasMoreDeploys || hasMoreEvents) && timeline.length ? (
            <View style={styles.loadMore}>
              <Button
                type="outline"
                loading={loadingMore}
                disabled={loadingMore}
                onPress={() => void loadMore().catch(() => undefined)}
                accessibilityLabel={t("activity.loadMore")}
              >
                {t("activity.loadMore")}
              </Button>
            </View>
          ) : null}
        </DashboardCard>
      </DashboardScrollView>
    </SafeAreaView>
  );
}

function serviceLifecycleResource(
  service: NonNullable<MobileServiceSupervisionQuery["service"]>,
): ServiceLifecycleResource {
  return {
    id: service.id ?? "",
    name: service.displayName || service.name || service.id || "Service",
    type: service.type || service.runtime || "unknown",
    phase: service.phase || "unknown",
    suspended: service.suspended,
    updatedAt: service.updatedAt,
    revision: service.revision,
    latestDeployId: service.latestDeployId,
  };
}

function ServiceIdentityCard({
  service,
}: {
  service: NonNullable<MobileServiceSupervisionQuery["service"]>;
}) {
  const { t } = useTranslations();
  const status =
    service.suspended === "suspended"
      ? "suspended"
      : service.phase || t("resources.unknownStatus");
  return (
    <DashboardCard title={t("service.overview")}>
      <View style={styles.identityStatus}>
        <StatusBadge status={status} />
      </View>
      <Detail label={t("service.region")} value={service.region} />
      <Detail
        label={t("service.replicas")}
        value={service.replicas == null ? null : String(service.replicas)}
      />
      <Detail label={t("service.revision")} value={service.revision} mono />
      <Detail
        label={t("service.latestDeploy")}
        value={service.latestDeployId}
        mono
      />
    </DashboardCard>
  );
}

function Detail({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: string | null | undefined;
  mono?: boolean;
}) {
  const theme = useTheme().colorTheme;
  return (
    <View style={[styles.detail, { borderTopColor: theme.border }]}>
      <Text style={[styles.detailLabel, { color: theme.mutedForeground }]}>
        {label}
      </Text>
      <Text
        numberOfLines={2}
        style={[
          styles.detailValue,
          { color: theme.foreground },
          mono ? { fontFamily: fonts.mono } : null,
        ]}
      >
        {value || "—"}
      </Text>
    </View>
  );
}

function TimelineRow({ item }: { item: TimelineItem }) {
  const { t, language } = useTranslations();
  const theme = useTheme().colorTheme;
  const isDeploy = item.kind === "deploy";
  const status = isDeploy
    ? item.deploy.status
    : (item.event.details?.deployStatus ?? item.event.details?.status);
  const eventType = isDeploy ? "deploy" : item.event.type;
  const title = isDeploy
    ? t("activity.deploy")
    : knownEventType(eventType)
      ? t(`events.types.${eventType}`)
      : t("events.unknown", { type: eventType || t("events.noType") });
  const commit = isDeploy
    ? item.deploy.commitMessage || item.deploy.commitId
    : item.event.details?.commitMessage || item.event.details?.commitId;
  const image = isDeploy ? item.deploy.image : item.event.details?.image;
  const transition = isDeploy ? null : eventTransition(item.event, t);
  return (
    <View style={[styles.timelineRow, { borderTopColor: theme.border }]}>
      <View
        style={[styles.timelineIcon, { backgroundColor: theme.primaryMuted }]}
      >
        <Ionicons
          name={isDeploy ? "rocket-outline" : "pulse-outline"}
          size={18}
          color={theme.primary}
        />
      </View>
      <View style={styles.timelineCopy}>
        <View style={styles.timelineTitleRow}>
          <Text style={[styles.timelineTitle, { color: theme.foreground }]}>
            {title}
          </Text>
          {status ? (
            <Text
              style={[
                styles.timelineStatus,
                { color: statusToneColor(status, theme) },
              ]}
            >
              {humanizeToken(status)}
            </Text>
          ) : null}
        </View>
        <Text style={[styles.timelineTime, { color: theme.mutedForeground }]}>
          {formatTimestamp(item.timestamp, language)}
        </Text>
        {transition ? (
          <Text style={[styles.timelineMeta, { color: theme.mutedForeground }]}>
            {transition}
          </Text>
        ) : null}
        {commit ? (
          <Text
            numberOfLines={2}
            style={[styles.timelineMeta, { color: theme.foreground }]}
          >
            {commit}
          </Text>
        ) : null}
        {image ? (
          <Text
            numberOfLines={1}
            style={[styles.timelineMono, { color: theme.mutedForeground }]}
          >
            {image}
          </Text>
        ) : null}
      </View>
    </View>
  );
}

function eventTransition(
  event: TimelineEvent,
  t: (key: string, options?: Record<string, unknown>) => string,
): string | null {
  const details = event.details;
  if (!details) return null;
  if (details.branchFrom || details.branchTo) {
    return t("events.branchTransition", {
      from: details.branchFrom ?? "—",
      to: details.branchTo ?? "—",
    });
  }
  if (details.fromCount != null || details.toCount != null) {
    return t("events.countTransition", {
      from: details.fromCount ?? "—",
      to: details.toCount ?? "—",
    });
  }
  if (details.reasonCode) {
    return t("events.reason", { reason: details.reasonCode });
  }
  return null;
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  content: { gap: space.md },
  notice: { borderWidth: 1, borderRadius: space.sm, padding: space.md },
  initialLoader: { minHeight: 120 },
  identityStatus: { paddingBottom: space.md },
  detail: {
    flexDirection: "row",
    gap: space.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingVertical: space.sm,
  },
  detailLabel: { width: 100, fontSize: fontSizes.sm },
  detailValue: { flex: 1, textAlign: "right", fontSize: fontSizes.sm },
  empty: {
    paddingHorizontal: space.lg,
    paddingVertical: space.xl,
    textAlign: "center",
  },
  timelineRow: {
    flexDirection: "row",
    gap: space.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: space.lg,
    paddingVertical: space.md,
  },
  timelineIcon: {
    width: 34,
    height: 34,
    borderRadius: 17,
    alignItems: "center",
    justifyContent: "center",
  },
  timelineCopy: { flex: 1, minWidth: 0 },
  timelineTitleRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: space.sm,
  },
  timelineTitle: {
    flex: 1,
    fontSize: fontSizes.md,
    fontWeight: fontWeights.medium,
  },
  timelineStatus: { fontSize: fontSizes.xs },
  timelineTime: { fontSize: fontSizes.xs, marginTop: space.xxs },
  timelineMeta: { fontSize: fontSizes.sm, marginTop: space.xs },
  timelineMono: {
    fontFamily: fonts.mono,
    fontSize: fontSizes.xs,
    marginTop: space.xs,
  },
  loadMore: { padding: space.lg },
  logBody: { marginBottom: space.md, lineHeight: 22 },
});

import { useEffect, useMemo, useState } from "react";
import { NetworkStatus } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import { router } from "expo-router";
import {
  ActivityIndicator,
  Pressable,
  StyleSheet,
  Text,
  View,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { Ionicons } from "@expo/vector-icons";
import { DashboardCard } from "@/components/dashboard-card";
import { DashboardScrollView } from "@/components/dashboard-scroll-view";
import { TopBar } from "@/components/top-bar";
import { useWorkspace } from "@/features/workspaces/workspace-provider";
import { useTranslations } from "@/common/hooks/use-translations";
import { useFreshness } from "@/common/hooks/freshness";
import {
  useRecovery,
  useRecoveryEnvironment,
} from "@/common/hooks/use-recovery";
import { recoveryAvailable } from "@/common/hooks/recovery-coordinator";
import { formatTimestamp, humanizeToken } from "@/common/format-util";
import {
  fontSizes,
  fontWeights,
  gutter,
  space,
  useTheme,
} from "@/common/theme";
import {
  MobileResourceStatusDocument,
  MobileUsageGlanceDocument,
} from "@/generated-graphql";
import {
  UsageGlanceCard,
  type UsageGlanceCopy,
} from "@/features/usage/usage-glance-card";
import {
  buildResourceGroups,
  filterResourceGroups,
  type ResourceKind,
  type ResourceStatusItem,
} from "./resource-groups";
import { StatusBadge } from "./status-badge";

const filters: (ResourceKind | "all")[] = [
  "all",
  "service",
  "database",
  "keyValue",
];

export function ResourceStatusScreen({
  activityOnly = false,
}: {
  activityOnly?: boolean;
}) {
  const { t, language } = useTranslations();
  const theme = useTheme().colorTheme;
  const { selected } = useWorkspace();
  const recoveryEnvironment = useRecoveryEnvironment();
  const [filter, setFilter] = useState<ResourceKind | "all">(
    activityOnly ? "service" : "all",
  );
  const [freshAt, setFreshAt] = useState<Date | null>(null);
  const { data, loading, error, refetch, networkStatus } = useQuery(
    MobileResourceStatusDocument,
    {
      variables: { ownerId: selected?.id ?? "" },
      skip: !selected,
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      notifyOnNetworkStatusChange: true,
      pollInterval: recoveryAvailable(recoveryEnvironment) ? 30_000 : 0,
    },
  );
  const usageQuery = useQuery(MobileUsageGlanceDocument, {
    variables: { ownerId: selected?.id ?? "" },
    skip: !selected || activityOnly,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    notifyOnNetworkStatusChange: true,
    pollInterval: recoveryAvailable(recoveryEnvironment) ? 60_000 : 0,
  });
  const recovery = useRecovery({
    attempt: async () => {
      await Promise.all([
        refetch(),
        activityOnly || !selected ? Promise.resolve() : usageQuery.refetch(),
      ]);
    },
  });
  const freshness = useFreshness(freshAt, { staleAfterMs: 65_000 });

  useEffect(() => {
    if (data && networkStatus === NetworkStatus.ready) setFreshAt(new Date());
  }, [data, networkStatus]);

  const grouped = useMemo(() => {
    const services: ResourceStatusItem[] = (data?.services ?? []).flatMap(
      (service) =>
        service?.id
          ? [
              {
                id: service.id,
                // "||" not "??": the API returns empty strings for unset names.
                name: service.displayName || service.name || service.id,
                kind: "service" as const,
                type: service.type || service.runtime || t("resources.service"),
                status:
                  service.suspended === "suspended"
                    ? "suspended"
                    : service.phase || t("resources.unknownStatus"),
                latestDeployId: service.latestDeployId ?? null,
                projectId: service.projectId ?? null,
                updatedAt: service.updatedAt ?? null,
              },
            ]
          : [],
    );
    const databases: ResourceStatusItem[] = (data?.databases ?? []).flatMap(
      (database) =>
        database?.id
          ? [
              {
                id: database.id,
                name: database.name || database.id,
                kind: "database" as const,
                type: database.version
                  ? `PostgreSQL ${database.version}`
                  : "PostgreSQL",
                status:
                  database.suspended === "suspended"
                    ? "suspended"
                    : (database.status ?? t("resources.unknownStatus")),
                latestDeployId: null,
                projectId: database.projectId ?? null,
                updatedAt: database.updatedAt ?? null,
              },
            ]
          : [],
    );
    const keyValues: ResourceStatusItem[] = (data?.keyValues ?? []).flatMap(
      (keyValue) =>
        keyValue?.id
          ? [
              {
                id: keyValue.id,
                name: keyValue.name || keyValue.id,
                kind: "keyValue" as const,
                type: keyValue.version
                  ? `Valkey ${keyValue.version}`
                  : "Valkey",
                status:
                  keyValue.suspended === "suspended"
                    ? "suspended"
                    : (keyValue.status ?? t("resources.unknownStatus")),
                latestDeployId: null,
                projectId: keyValue.projectId ?? null,
                updatedAt: keyValue.updatedAt ?? null,
              },
            ]
          : [],
    );
    const projects = (data?.projects ?? []).flatMap((project) =>
      project?.id
        ? [
            {
              id: project.id,
              name: project.name || project.id,
              serviceIds: project.serviceIds,
              databaseIds: project.databaseIds,
              keyValueIds: project.keyValueIds,
            },
          ]
        : [],
    );
    return filterResourceGroups(
      buildResourceGroups(projects, [...services, ...databases, ...keyValues]),
      filter,
    );
  }, [data, filter, t]);

  const shownGroups = grouped.groups.filter((group) => group.resources.length);
  const resourceCount =
    shownGroups.reduce((sum, group) => sum + group.resources.length, 0) +
    grouped.ungrouped.length;
  const initialLoading = loading && !data;
  const refreshing = networkStatus === NetworkStatus.refetch;
  const usageCopy = useMemo<UsageGlanceCopy>(
    () => ({
      title: t("usageGlance.title"),
      states: {
        complete: t("usageGlance.states.complete"),
        partial: t("usageGlance.states.partial"),
        unknown: t("usageGlance.states.unknown"),
        unavailable: t("usageGlance.states.unavailable"),
        "healthy-empty": t("usageGlance.states.healthy-empty"),
      },
      meterLabels: {
        instance_seconds: t("usageGlance.meters.instance_seconds"),
        egress_bytes: t("usageGlance.meters.egress_bytes"),
        build_seconds: t("usageGlance.meters.build_seconds"),
        storage_gb_seconds: t("usageGlance.meters.storage_gb_seconds"),
        sandbox_compute_seconds: t(
          "usageGlance.meters.sandbox_compute_seconds",
        ),
      },
      empty: t("usageGlance.empty"),
      noEvidence: t("usageGlance.noEvidence"),
      through: (timestamp) =>
        t("usageGlance.through", {
          time: formatTimestamp(timestamp, language),
        }),
      degraded: (sources) => t("usageGlance.degraded", { sources }),
      refreshUnavailable: t("usageGlance.refreshUnavailable"),
    }),
    [language, t],
  );

  return (
    <SafeAreaView style={[styles.safe, { backgroundColor: theme.background }]}>
      <TopBar
        title={t(activityOnly ? "activity.title" : "status.title")}
        right={
          loading && data ? <ActivityIndicator color={theme.primary} /> : null
        }
      />
      <DashboardScrollView
        refreshing={
          refreshing || usageQuery.networkStatus === NetworkStatus.refetch
        }
        onRefresh={() => void recovery.manualRetry()}
        contentContainerStyle={styles.content}
      >
        <Text style={[styles.freshness, { color: theme.mutedForeground }]}>
          {freshAt
            ? `${t(freshness.label)} · ${t("resources.updatedAt", {
                time: formatTimestamp(freshAt.toISOString(), language),
              })}`
            : t(freshness.label)}
        </Text>
        {!activityOnly ? (
          usageQuery.loading && !usageQuery.data && !usageQuery.previousData ? (
            <DashboardCard title={t("usageGlance.title")}>
              <ActivityIndicator
                accessibilityLabel={t("usageGlance.loading")}
                color={theme.primary}
              />
            </DashboardCard>
          ) : (
            <UsageGlanceCard
              summary={usageQuery.data?.usage ?? usageQuery.previousData?.usage}
              unavailable={Boolean(usageQuery.error)}
              copy={usageCopy}
            />
          )
        ) : null}
        {!activityOnly ? (
          <View
            accessibilityRole="tablist"
            accessibilityLabel={t("resources.filterLabel")}
            style={styles.filters}
          >
            {filters.map((candidate) => {
              const selectedFilter = candidate === filter;
              return (
                <Pressable
                  key={candidate}
                  accessibilityRole="tab"
                  accessibilityState={{ selected: selectedFilter }}
                  accessibilityLabel={t(`resources.filters.${candidate}`)}
                  onPress={() => setFilter(candidate)}
                  style={[
                    styles.filter,
                    {
                      backgroundColor: selectedFilter
                        ? theme.primaryMuted
                        : theme.card,
                      borderColor: selectedFilter
                        ? theme.primary
                        : theme.border,
                    },
                  ]}
                >
                  <Text
                    style={{
                      color: selectedFilter
                        ? theme.primary
                        : theme.mutedForeground,
                      fontWeight: fontWeights.medium,
                    }}
                  >
                    {t(`resources.filters.${candidate}`)}
                  </Text>
                </Pressable>
              );
            })}
          </View>
        ) : (
          <Text style={[styles.activityHint, { color: theme.mutedForeground }]}>
            {t("activity.body")}
          </Text>
        )}

        {error ? (
          <View
            accessibilityRole="alert"
            style={[styles.notice, { borderColor: theme.warning }]}
          >
            <Text style={{ color: theme.warning }}>
              {data ? t("resources.partialError") : t("resources.loadError")}
            </Text>
          </View>
        ) : null}

        {initialLoading ? (
          <DashboardCard>
            <View style={styles.loading}>
              <ActivityIndicator color={theme.primary} />
              <Text style={{ color: theme.mutedForeground }}>
                {t("resources.loading")}
              </Text>
            </View>
          </DashboardCard>
        ) : resourceCount === 0 ? (
          <DashboardCard>
            <Text style={[styles.emptyTitle, { color: theme.foreground }]}>
              {t("resources.emptyTitle")}
            </Text>
            <Text style={{ color: theme.mutedForeground }}>
              {filter === "all"
                ? t("resources.emptyBody")
                : t("resources.emptyFilter")}
            </Text>
          </DashboardCard>
        ) : (
          <>
            {shownGroups.map((group) => (
              <ResourceGroupCard
                key={group.id}
                name={group.name}
                resources={group.resources}
              />
            ))}
            {grouped.ungrouped.length ? (
              <ResourceGroupCard
                name={t("resources.ungrouped")}
                resources={grouped.ungrouped}
              />
            ) : null}
          </>
        )}
      </DashboardScrollView>
    </SafeAreaView>
  );
}

function ResourceGroupCard({
  name,
  resources,
}: {
  name: string;
  resources: ResourceStatusItem[];
}) {
  return (
    <DashboardCard title={name} bleed>
      {resources.map((resource) => (
        <ResourceRow
          key={`${resource.kind}:${resource.id}`}
          resource={resource}
        />
      ))}
    </DashboardCard>
  );
}

function ResourceRow({ resource }: { resource: ResourceStatusItem }) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const canOpen = true;
  const href =
    resource.kind === "service"
      ? `/services/${encodeURIComponent(resource.id)}`
      : resource.kind === "database"
        ? `/databases/${encodeURIComponent(resource.id)}`
        : `/key-values/${encodeURIComponent(resource.id)}`;
  return (
    <Pressable
      disabled={!canOpen}
      accessibilityRole={canOpen ? "link" : "text"}
      accessibilityLabel={t("resources.resourceAccessibility", {
        name: resource.name,
        type: resource.type,
        status: resource.status,
      })}
      onPress={() => router.push(href)}
      style={({ pressed }) => [
        styles.resourceRow,
        { borderTopColor: theme.border, opacity: pressed ? 0.65 : 1 },
      ]}
    >
      <View style={[styles.kindIcon, { backgroundColor: theme.primaryMuted }]}>
        <Ionicons
          name={
            resource.kind === "service"
              ? "cube-outline"
              : resource.kind === "database"
                ? "server-outline"
                : "flash-outline"
          }
          size={20}
          color={theme.primary}
        />
      </View>
      <View style={styles.resourceCopy}>
        <Text
          numberOfLines={1}
          style={[styles.resourceName, { color: theme.foreground }]}
        >
          {resource.name}
        </Text>
        <Text
          numberOfLines={1}
          style={[styles.meta, { color: theme.mutedForeground }]}
        >
          {humanizeToken(resource.type)}
        </Text>
        {resource.latestDeployId ? (
          <Text
            numberOfLines={1}
            style={[styles.meta, { color: theme.mutedForeground }]}
          >
            {t("resources.latestDeploy", { id: resource.latestDeployId })}
          </Text>
        ) : null}
      </View>
      <View style={styles.statusColumn}>
        <StatusBadge status={resource.status} compact />
        {canOpen ? (
          <Ionicons
            name="chevron-forward"
            size={18}
            color={theme.mutedForeground}
          />
        ) : null}
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  content: { gap: space.md },
  freshness: { fontSize: fontSizes.sm, marginBottom: space.xs },
  activityHint: {
    fontSize: fontSizes.md,
    lineHeight: fontSizes.md * 1.5,
    color: undefined,
  },
  filters: { flexDirection: "row", flexWrap: "wrap", gap: space.sm },
  filter: {
    minHeight: 36,
    justifyContent: "center",
    borderWidth: 1,
    borderRadius: 999,
    paddingHorizontal: space.md,
    paddingVertical: space.xs,
  },
  notice: { borderWidth: 1, borderRadius: space.sm, padding: space.md },
  loading: {
    minHeight: 140,
    alignItems: "center",
    justifyContent: "center",
    gap: space.sm,
  },
  emptyTitle: {
    fontSize: fontSizes.lg,
    fontWeight: fontWeights.medium,
    marginBottom: space.xs,
  },
  resourceRow: {
    minHeight: 72,
    flexDirection: "row",
    alignItems: "center",
    paddingHorizontal: gutter,
    paddingVertical: space.sm,
    borderTopWidth: StyleSheet.hairlineWidth,
    gap: space.md,
  },
  kindIcon: {
    width: 36,
    height: 36,
    borderRadius: 10,
    alignItems: "center",
    justifyContent: "center",
  },
  resourceCopy: { flex: 1, minWidth: 0 },
  resourceName: { fontSize: fontSizes.md, fontWeight: fontWeights.medium },
  meta: { fontSize: fontSizes.sm, marginTop: 2 },
  statusColumn: {
    flexShrink: 1,
    maxWidth: 128,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "flex-end",
    gap: space.xs,
  },
});

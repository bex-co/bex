import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
} from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { ActivityIndicator, StyleSheet, Text, View } from "react-native";
import { Button } from "@/components/button";
import { DashboardCard } from "@/components/dashboard-card";
import {
  SafeActionPanel,
  defineSafeAction,
  type MobileActionOption,
  type MobileActionRunResult,
} from "@/components/safe-action";
import { formatTimestamp } from "@/common/format-util";
import { dataBoundary } from "@/common/apollo/data-boundary";
import { hasAuthoritativeCurrentEvidence } from "@/common/apollo/authoritative-evidence";
import { useTranslations } from "@/common/hooks/use-translations";
import { fonts, fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import {
  MobileCancelCronRunDocument,
  MobileCronRunsDocument,
  MobileRunCronJobDocument,
  type MobileCronRunsQuery,
} from "@/generated-graphql";
import { isLifecycleSuspended } from "@/features/services/lifecycle";
import {
  CronActionController,
  awaitCronActionObservation,
  type CronActionRequest,
} from "./cron-action-controller";
import {
  cronRunDuration,
  composeCronRunPages,
  currentCronRun,
  isActiveCronRun,
  mergeCronRuns,
  type CronRunSummary,
} from "./cron-history";

const PAGE_SIZE = 10;
const runCronNow = defineSafeAction("run-cron-job", "service");
const cancelCronRun = defineSafeAction("cancel-cron-run", "cron-run");

export type CronRunsCardHandle = { refresh: () => Promise<void> };

export const CronRunsCard = forwardRef<
  CronRunsCardHandle,
  {
    serviceId: string;
    serviceLabel: string;
    suspended: boolean | string | null;
    serviceEvidenceCurrent: boolean;
    onOpenLogs: () => void;
  }
>(function CronRunsCard(
  { serviceId, serviceLabel, suspended, serviceEvidenceCurrent, onOpenLogs },
  ref,
) {
  const { t, language } = useTranslations();
  const theme = useTheme().colorTheme;
  const [extraRuns, setExtraRuns] = useState<CronRunSummary[]>([]);
  const [hasMore, setHasMore] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [refreshRequired, setRefreshRequired] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [timeoutKind, setTimeoutKind] = useState<
    "ambiguous" | "convergence" | null
  >(null);
  const [panelEpoch, setPanelEpoch] = useState(0);
  const boundaryRef = useRef(0);
  const serviceIdRef = useRef(serviceId);
  serviceIdRef.current = serviceId;
  const query = useQuery(MobileCronRunsDocument, {
    variables: { serviceId, limit: PAGE_SIZE },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    notifyOnNetworkStatusChange: true,
    pollInterval: 30_000,
  });
  const [runNow] = useMutation(MobileRunCronJobDocument);
  const [cancelRun] = useMutation(MobileCancelCronRunDocument);

  const firstPage = useMemo(
    () => cronRuns(query.data?.cronJobRuns),
    [query.data?.cronJobRuns],
  );
  const runs = useMemo(
    () => composeCronRunPages(firstPage, extraRuns),
    [extraRuns, firstPage],
  );
  const suspendedRef = useRef(suspended);
  suspendedRef.current = suspended;

  const controllerRef = useRef<CronActionController | null>(null);
  const mutationsRef = useRef({ runNow, cancelRun });
  mutationsRef.current = { runNow, cancelRun };
  if (!controllerRef.current) {
    controllerRef.current = new CronActionController({
      run: async (id, signal) => {
        const result = await mutationsRef.current.runNow({
          variables: { id },
          context: { fetchOptions: { signal } },
        });
        return { run: result.data?.runCronJob };
      },
      cancel: async (id, runId, signal) => {
        const result = await mutationsRef.current.cancelRun({
          variables: { serviceId: id, runId },
          context: { fetchOptions: { signal } },
        });
        return { run: result.data?.cancelCronJobRun };
      },
    });
  }

  useEffect(() => {
    boundaryRef.current += 1;
    setExtraRuns([]);
    setHasMore(true);
    setRefreshRequired(false);
    setTimeoutKind(null);
    return () => {
      boundaryRef.current += 1;
      controllerRef.current?.clear();
    };
  }, [serviceId]);

  useEffect(
    () =>
      dataBoundary.registerResetHandler(() => {
        boundaryRef.current += 1;
        controllerRef.current?.clear();
        setExtraRuns([]);
        setHasMore(true);
        setLoadingMore(false);
        // Fail closed until the new identity/workspace proves current history.
        setRefreshRequired(true);
        setRefreshing(false);
        setTimeoutKind(null);
        setPanelEpoch((current) => current + 1);
      }),
    [],
  );

  useEffect(() => {
    if (!query.loading && extraRuns.length === 0) {
      setHasMore(firstPage.length === PAGE_SIZE);
    }
  }, [extraRuns.length, firstPage.length, query.loading]);

  async function refreshHistory(
    expectedBoundary = boundaryRef.current,
    expectedService = serviceId,
  ): Promise<CronRunSummary[]> {
    const result = await query.refetch({
      serviceId: expectedService,
      limit: PAGE_SIZE,
    });
    if (
      boundaryRef.current !== expectedBoundary ||
      serviceIdRef.current !== expectedService
    ) {
      throw new Error("cron data boundary changed");
    }
    const refreshed = cronRuns(result.data?.cronJobRuns);
    setExtraRuns([]);
    setHasMore(refreshed.length === PAGE_SIZE);
    return refreshed;
  }

  async function authoritativeRefresh() {
    const boundary = boundaryRef.current;
    const requestedService = serviceId;
    setRefreshing(true);
    try {
      await refreshHistory(boundary, requestedService);
      if (
        boundaryRef.current !== boundary ||
        serviceIdRef.current !== requestedService
      ) {
        return;
      }
      setRefreshRequired(false);
      setTimeoutKind(null);
      controllerRef.current?.markAuthoritativelyRefreshed(requestedService);
    } finally {
      if (
        boundaryRef.current === boundary &&
        serviceIdRef.current === requestedService
      ) {
        setRefreshing(false);
      }
    }
  }

  useImperativeHandle(ref, () => ({ refresh: authoritativeRefresh }));

  async function loadMore() {
    const boundary = boundaryRef.current;
    const requestedService = serviceId;
    const cursor = runs.at(-1)?.id;
    if (!cursor || !hasMore || loadingMore) return;
    setLoadingMore(true);
    try {
      const result = await query.fetchMore({
        variables: { serviceId: requestedService, cursor, limit: PAGE_SIZE },
      });
      if (
        boundaryRef.current !== boundary ||
        serviceIdRef.current !== requestedService
      ) {
        return;
      }
      const page = cronRuns(result.data?.cronJobRuns);
      setExtraRuns((current) => mergeCronRuns(current, page));
      setHasMore(page.length === PAGE_SIZE);
    } finally {
      if (
        boundaryRef.current === boundary &&
        serviceIdRef.current === requestedService
      ) {
        setLoadingMore(false);
      }
    }
  }

  async function execute(
    request: CronActionRequest,
  ): Promise<MobileActionRunResult> {
    const boundary = boundaryRef.current;
    const result = await controllerRef.current!.execute(request);
    if (boundaryRef.current !== boundary) return { status: "not_allowed" };
    if (result.outcome === "rejected") {
      if (result.refreshRequired) setRefreshRequired(true);
      return { status: "error", error: result.error };
    }
    try {
      const observation = await awaitCronActionObservation({
        request,
        result,
        refresh: async () => {
          if (boundaryRef.current !== boundary) {
            throw new Error("cron data boundary changed");
          }
          const refreshed = await refreshHistory(boundary, request.serviceId);
          if (boundaryRef.current !== boundary) {
            throw new Error("cron data boundary changed");
          }
          return refreshed;
        },
      });
      if (boundaryRef.current !== boundary) return { status: "not_allowed" };
      if (observation.observed) {
        setRefreshRequired(false);
        setTimeoutKind(null);
        controllerRef.current?.markAuthoritativelyRefreshed(request.serviceId);
        return { status: "success" };
      }
    } catch {
      // A failed authoritative read cannot make an uncertain mutation safe to
      // repeat. The explicit refresh control below is the recovery boundary.
    }
    if (boundaryRef.current !== boundary) return { status: "not_allowed" };
    controllerRef.current?.requireAuthoritativeRefresh(request.serviceId);
    setRefreshRequired(true);
    setTimeoutKind(result.outcome === "unknown" ? "ambiguous" : "convergence");
    return { status: "timeout" };
  }

  const activeRun = currentCronRun(runs);
  const actionOptions: MobileActionOption[] = [];
  const historyEvidenceCurrent = hasAuthoritativeCurrentEvidence({
    networkStatus: query.networkStatus,
    error: query.error,
    hasData: query.data !== undefined,
  });
  if (!refreshRequired && historyEvidenceCurrent && serviceEvidenceCurrent) {
    if (!isLifecycleSuspended(suspended)) {
      actionOptions.push({
        key: "cron:run",
        definition: runCronNow,
        target: { kind: "service", id: serviceId, label: serviceLabel },
        label: t("cron.runNow"),
        run: (_confirmation, retryIdentity) =>
          execute({
            requestId: retryIdentity!,
            action: "run",
            serviceId,
            serviceSuspended: isLifecycleSuspended(suspendedRef.current),
          }),
      });
    }
    if (activeRun) {
      actionOptions.push({
        key: `cron:cancel:${activeRun.id}`,
        definition: cancelCronRun,
        target: { kind: "cron-run", id: activeRun.id, label: activeRun.id },
        label: t("cron.cancelCurrent"),
        run: (_confirmation, retryIdentity) =>
          execute({
            requestId: retryIdentity!,
            action: "cancel",
            serviceId,
            serviceSuspended: isLifecycleSuspended(suspendedRef.current),
            target: activeRun,
          }),
      });
    }
  }

  const initialLoading = query.loading && runs.length === 0;
  return (
    <DashboardCard title={t("cron.title")}>
      <Text style={[styles.description, { color: theme.mutedForeground }]}>
        {t("cron.description")}
      </Text>

      {isLifecycleSuspended(suspended) ? (
        <Text style={[styles.notice, { color: theme.warning }]}>
          {t("cron.suspended")}
        </Text>
      ) : null}
      {refreshRequired ? (
        <View style={styles.refreshBoundary} accessibilityRole="alert">
          <Text style={{ color: theme.warning }}>
            {t("cron.refreshRequired")}
          </Text>
          <Button
            type="outline"
            loading={refreshing}
            disabled={refreshing}
            onPress={() => void authoritativeRefresh().catch(() => undefined)}
            accessibilityLabel={t("cron.refresh")}
          >
            {refreshing ? t("cron.refreshing") : t("cron.refresh")}
          </Button>
        </View>
      ) : null}

      <SafeActionPanel
        key={`${serviceId}:${panelEpoch}`}
        options={actionOptions}
        feedbackMessages={
          timeoutKind
            ? {
                "timeout-unknown":
                  timeoutKind === "ambiguous"
                    ? t("cron.feedback.ambiguous")
                    : t("cron.feedback.convergenceTimeout"),
              }
            : undefined
        }
      />

      <View style={[styles.divider, { borderTopColor: theme.border }]} />
      {initialLoading ? (
        <ActivityIndicator color={theme.primary} style={styles.loader} />
      ) : query.error && runs.length === 0 ? (
        <Text style={{ color: theme.error }}>{t("cron.loadError")}</Text>
      ) : runs.length === 0 ? (
        <Text style={{ color: theme.mutedForeground }}>{t("cron.empty")}</Text>
      ) : (
        <View>
          {runs.map((run) => (
            <CronRunRow key={run.id} run={run} language={language} />
          ))}
          {hasMore ? (
            <Button
              type="outline"
              style={styles.loadMore}
              loading={loadingMore}
              disabled={loadingMore}
              onPress={() => void loadMore().catch(() => undefined)}
              accessibilityLabel={t("cron.loadMore")}
            >
              {t("cron.loadMore")}
            </Button>
          ) : null}
        </View>
      )}

      <Button
        type="outline"
        style={styles.logs}
        onPress={onOpenLogs}
        accessibilityLabel={t("cron.openLogs")}
      >
        {t("cron.openLogs")}
      </Button>
    </DashboardCard>
  );
});

function CronRunRow({
  run,
  language,
}: {
  run: CronRunSummary;
  language: string;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const duration = cronRunDuration(run);
  const active = isActiveCronRun(run.status);
  return (
    <View style={[styles.row, { borderTopColor: theme.border }]}>
      <View style={styles.rowCopy}>
        <Text
          style={[styles.runId, { color: theme.foreground }]}
          numberOfLines={1}
        >
          {run.id}
        </Text>
        <Text style={[styles.timing, { color: theme.mutedForeground }]}>
          {t("cron.started", {
            time: run.startedAt
              ? formatTimestamp(run.startedAt, language)
              : t("cron.notStarted"),
          })}
        </Text>
        {run.finishedAt ? (
          <Text style={[styles.timing, { color: theme.mutedForeground }]}>
            {t("cron.finished", {
              time: formatTimestamp(run.finishedAt, language),
            })}
          </Text>
        ) : null}
        {duration ? (
          <Text style={[styles.timing, { color: theme.mutedForeground }]}>
            {t("cron.duration", { duration })}
          </Text>
        ) : null}
      </View>
      <Text
        style={[
          styles.status,
          { color: active ? theme.warning : theme.mutedForeground },
        ]}
      >
        {t(`cron.status.${cronStatusKey(run.status)}`)}
      </Text>
    </View>
  );
}

function cronRuns(
  values: MobileCronRunsQuery["cronJobRuns"] | undefined,
): CronRunSummary[] {
  return (values ?? []).flatMap((run) =>
    run?.id?.trim()
      ? [
          {
            id: run.id.trim(),
            status: run.status?.trim() || "unknown",
            startedAt: run.startedAt ?? null,
            finishedAt: run.finishedAt ?? null,
          },
        ]
      : [],
  );
}

function cronStatusKey(
  status: string,
): "pending" | "running" | "successful" | "failed" | "canceled" | "unknown" {
  switch (status.toLowerCase()) {
    case "pending":
      return "pending";
    case "running":
      return "running";
    case "successful":
    case "succeeded":
      return "successful";
    case "failed":
    case "unsuccessful":
      return "failed";
    case "canceled":
      return "canceled";
    default:
      return "unknown";
  }
}

const styles = StyleSheet.create({
  description: { lineHeight: 21, marginBottom: space.md },
  notice: { marginBottom: space.md },
  refreshBoundary: { gap: space.sm, marginBottom: space.md },
  divider: {
    borderTopWidth: StyleSheet.hairlineWidth,
    marginTop: space.md,
  },
  loader: { minHeight: 88 },
  row: {
    flexDirection: "row",
    alignItems: "flex-start",
    gap: space.md,
    borderTopWidth: StyleSheet.hairlineWidth,
    paddingVertical: space.md,
  },
  rowCopy: { flex: 1, minWidth: 0 },
  runId: {
    fontFamily: fonts.mono,
    fontSize: fontSizes.xs,
    fontWeight: fontWeights.medium,
  },
  timing: { fontSize: fontSizes.xs, marginTop: space.xxs },
  status: { fontSize: fontSizes.sm },
  loadMore: { marginTop: space.sm },
  logs: { marginTop: space.md },
});

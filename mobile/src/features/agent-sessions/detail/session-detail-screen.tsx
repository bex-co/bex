import { useEffect, useRef, useState } from "react";
import { NetworkStatus } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import { Linking, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { DashboardCard } from "@/components/dashboard-card";
import { DashboardScrollView } from "@/components/dashboard-scroll-view";
import { DetailHeader } from "@/components/detail-header";
import { Button } from "@/components/button";
import {
  SafeActionPanel,
  defineSafeAction,
  type MobileActionOption,
} from "@/components/safe-action";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useRecovery,
  useRecoveryEnvironment,
} from "@/common/hooks/use-recovery";
import { formatTimestamp } from "@/common/format-util";
import { fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import { dataBoundary } from "@/common/apollo/data-boundary";
import { AccessRequiredScreen } from "@/features/capabilities/access-required-screen";
import { useCapabilities } from "@/features/capabilities/capabilities-provider";
import {
  MobileAgentSessionDocument,
  MobileCancelAgentSessionDocument,
} from "@/generated-graphql";
import { isCancelablePhase, sessionPhaseView } from "../lifecycle";
import { isGitHubPrUrl } from "./github-links";
import { sessionDetailPollIntervalMs } from "./session-detail-refresh";

const cancelAgentSession = defineSafeAction(
  "cancel-agent-session",
  "agent-session",
);

export function SessionDetailScreen({ sessionId }: { sessionId: string }) {
  const { t, language } = useTranslations();
  const theme = useTheme().colorTheme;
  const capabilities = useCapabilities();
  const recoveryEnvironment = useRecoveryEnvironment();
  // ADR087: the session read is gated on confirmed can_operate — a deep link
  // without it renders the generic access state before any fetch, and never
  // echoes whether the id exists.
  const canReadSessions = capabilities.allows("can_operate");
  const generation = capabilities.generation;
  const observedSessionId = useRef(sessionId);
  observedSessionId.current = sessionId;
  // Until the first response lands, treat the session as active so a direct
  // deep link into a running session still polls. Terminal phases stop it.
  const [knownPhase, setKnownPhase] = useState<string | null | undefined>(
    undefined,
  );

  useEffect(() => {
    setKnownPhase(undefined);
  }, [sessionId]);

  const { data, loading, error, refetch, networkStatus } = useQuery(
    MobileAgentSessionDocument,
    {
      variables: { id: sessionId },
      skip: !canReadSessions,
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      notifyOnNetworkStatusChange: true,
      // Bound cadence for active sessions only; terminal + background/offline
      // stop. Direct deep links cannot rely on the list's poll to advance phase.
      pollInterval: canReadSessions
        ? sessionDetailPollIntervalMs(knownPhase, recoveryEnvironment)
        : 0,
    },
  );

  useEffect(() => {
    setKnownPhase(data?.agentSession?.phase);
  }, [data?.agentSession?.phase]);

  const recovery = useRecovery({
    attempt: async ({ signal }) => {
      if (!canReadSessions) return;
      if (signal.aborted || observedSessionId.current !== sessionId) return;
      const lease = dataBoundary.begin();
      try {
        if (!lease.isCurrent() || lease.signal.aborted) return;
        await refetch();
      } finally {
        lease.finish();
      }
    },
  });

  useEffect(() => {
    // Session / access generation changes must not leave a prior detail's
    // in-flight recovery writing into the new destination.
    recovery.cancel();
  }, [sessionId, generation, recovery.cancel]);

  const [cancelSession] = useMutation(MobileCancelAgentSessionDocument);

  if (!canReadSessions) {
    return <AccessRequiredScreen action="can_operate" />;
  }

  const session = data?.agentSession ?? null;
  const phase = sessionPhaseView(session?.phase);
  const refreshFailed = Boolean(error && session);
  const staleCached =
    recovery.phase === "failed" && session != null && !refreshFailed;

  const options: MobileActionOption[] =
    session && isCancelablePhase(session.phase)
      ? [
          {
            key: `agent:cancel:${session.id}`,
            definition: cancelAgentSession,
            target: {
              kind: "agent-session",
              id: session.id,
              label: session.repo,
            },
            label: t("agentSessions.detail.cancel"),
            run: async () => {
              try {
                await cancelSession({ variables: { id: session.id } });
                return { status: "accepted_unverified" };
              } catch (cancelError) {
                return { status: "error", error: cancelError };
              }
            },
          },
        ]
      : [];

  return (
    <SafeAreaView
      edges={["top", "bottom", "left", "right"]}
      style={[styles.safe, { backgroundColor: theme.background }]}
    >
      <DetailHeader
        title={session?.repo ?? sessionId}
        subtitle={session?.branch ?? ""}
      />
      <DashboardScrollView
        refreshing={networkStatus === NetworkStatus.refetch}
        onRefresh={() => void recovery.manualRetry()}
        contentContainerStyle={styles.content}
      >
        {refreshFailed || staleCached ? (
          <Text
            accessibilityRole="alert"
            style={[styles.notice, { color: theme.warning }]}
          >
            {t(
              refreshFailed
                ? "agentSessions.detail.refreshError"
                : "agentSessions.detail.stale",
            )}
          </Text>
        ) : null}
        {loading && !data ? null : error && !session ? (
          <DashboardCard>
            <Text
              accessibilityRole="alert"
              style={[styles.body, { color: theme.mutedForeground }]}
            >
              {t("agentSessions.detail.unavailable")}
            </Text>
          </DashboardCard>
        ) : !session ? (
          <DashboardCard>
            <Text style={[styles.body, { color: theme.mutedForeground }]}>
              {t("agentSessions.detail.notFound")}
            </Text>
          </DashboardCard>
        ) : (
          <>
            <DashboardCard title={t("agentSessions.detail.overview")}>
              <View style={styles.row}>
                <Text style={[styles.body, { color: theme.mutedForeground }]}>
                  {t("agentSessions.detail.phase")}
                </Text>
                <Text
                  style={[
                    styles.phase,
                    {
                      color:
                        phase.tone === "danger"
                          ? theme.warning
                          : phase.tone === "active"
                            ? theme.primary
                            : theme.foreground,
                    },
                  ]}
                >
                  {t(phase.labelKey)}
                </Text>
              </View>
              <Text style={[styles.meta, { color: theme.mutedForeground }]}>
                {t("agentSessions.detail.updated", {
                  time: formatTimestamp(session.updatedAt, language),
                })}
              </Text>
            </DashboardCard>

            {session.phase === "failed" && session.failureReason ? (
              <DashboardCard title={t("agentSessions.detail.failure")}>
                <Text
                  accessibilityRole="alert"
                  style={[styles.body, { color: theme.warning }]}
                >
                  {session.failureReason}
                </Text>
              </DashboardCard>
            ) : null}

            {isGitHubPrUrl(session.prUrl) ? (
              <DashboardCard title={t("agentSessions.detail.pullRequest")}>
                <Text style={[styles.body, { color: theme.mutedForeground }]}>
                  {session.prNumber
                    ? `#${session.prNumber}`
                    : t("agentSessions.detail.draftPr")}
                </Text>
                <Button
                  type="outline"
                  onPress={() => void Linking.openURL(session.prUrl!)}
                  accessibilityLabel={t("agentSessions.detail.openPr")}
                >
                  {t("agentSessions.detail.openPr")}
                </Button>
              </DashboardCard>
            ) : null}

            <SafeActionPanel options={options} />
          </>
        )}
      </DashboardScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  content: { gap: space.md },
  row: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  body: { fontSize: fontSizes.sm, lineHeight: fontSizes.sm * 1.5 },
  meta: { fontSize: fontSizes.xs, marginTop: space.xs },
  phase: { fontSize: fontSizes.sm, fontWeight: fontWeights.medium },
  notice: { fontSize: fontSizes.sm, lineHeight: fontSizes.sm * 1.4 },
});

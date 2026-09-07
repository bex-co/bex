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
import { formatTimestamp } from "@/common/format-util";
import { fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import { AccessRequiredScreen } from "@/features/capabilities/access-required-screen";
import { useCapabilities } from "@/features/capabilities/capabilities-provider";
import {
  MobileAgentSessionDocument,
  MobileCancelAgentSessionDocument,
} from "@/generated-graphql";
import { isCancelablePhase, sessionPhaseView } from "../lifecycle";
import { isGitHubPrUrl } from "./github-links";

const cancelAgentSession = defineSafeAction(
  "cancel-agent-session",
  "agent-session",
);

export function SessionDetailScreen({ sessionId }: { sessionId: string }) {
  const { t, language } = useTranslations();
  const theme = useTheme().colorTheme;
  const capabilities = useCapabilities();
  // ADR087: the session read is gated on confirmed can_operate — a deep link
  // without it renders the generic access state before any fetch, and never
  // echoes whether the id exists.
  const canReadSessions = capabilities.allows("can_operate");
  const { data, loading, error, refetch, networkStatus } = useQuery(
    MobileAgentSessionDocument,
    {
      variables: { id: sessionId },
      skip: !canReadSessions,
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      notifyOnNetworkStatusChange: true,
    },
  );
  const [cancelSession] = useMutation(MobileCancelAgentSessionDocument);

  if (!canReadSessions) {
    return <AccessRequiredScreen action="can_operate" />;
  }

  const session = data?.agentSession ?? null;
  const phase = sessionPhaseView(session?.phase);

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
        onRefresh={() => void refetch()}
        contentContainerStyle={styles.content}
      >
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
});

import { NetworkStatus } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  ActivityIndicator,
  Linking,
  StyleSheet,
  Text,
  View,
} from "react-native";
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
import {
  fontSizes,
  fontWeights,
  gutter,
  space,
  useTheme,
} from "@/common/theme";
import {
  MobileAgentSessionDocument,
  MobileCancelAgentSessionDocument,
} from "@/generated-graphql";
import { isCancelablePhase, sessionPhaseView } from "../lifecycle";
import { SessionConversation } from "../conversation/session-conversation";
import { failureReasonText, failureStatusText } from "./failure-reason";
import { isGitHubPrUrl } from "./github-links";

const cancelAgentSession = defineSafeAction(
  "cancel-agent-session",
  "agent-session",
);

export function SessionDetailScreen({ sessionId }: { sessionId: string }) {
  const { t, language } = useTranslations();
  const theme = useTheme().colorTheme;
  const { data, loading, error, refetch, networkStatus } = useQuery(
    MobileAgentSessionDocument,
    {
      variables: { id: sessionId },
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      notifyOnNetworkStatusChange: true,
    },
  );
  const [cancelSession] = useMutation(MobileCancelAgentSessionDocument);

  const session = data?.agentSession ?? null;
  const phase = sessionPhaseView(session?.phase);
  const failureReason =
    failureReasonText(session?.failureReason) ??
    failureStatusText(session?.status);

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
    <SafeAreaView style={[styles.safe, { backgroundColor: theme.background }]}>
      <DetailHeader
        title={session?.repo ?? sessionId}
        subtitle={session?.branch ?? ""}
      />
      {session ? (
        <SessionConversation
          sessionId={session.id}
          liveEnabled={Boolean(session.sandboxId)}
          refreshing={networkStatus === NetworkStatus.refetch}
          onRefresh={() => void refetch()}
          header={
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
          }
          footer={
            <View style={styles.footer}>
              {session.phase === "failed" ? (
                <DashboardCard title={t("agentSessions.detail.failure")}>
                  <Text
                    accessibilityRole="alert"
                    style={[styles.body, { color: theme.warning }]}
                  >
                    {failureReason ?? t("agentSessions.detail.failureUnknown")}
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
            </View>
          }
        />
      ) : (
        <DashboardScrollView
          refreshing={networkStatus === NetworkStatus.refetch}
          onRefresh={() => void refetch()}
          contentContainerStyle={styles.content}
        >
          {loading && !data ? (
            <DashboardCard>
              <ActivityIndicator color={theme.primary} />
            </DashboardCard>
          ) : error ? (
            <DashboardCard>
              <Text
                accessibilityRole="alert"
                style={[styles.body, { color: theme.mutedForeground }]}
              >
                {t("agentSessions.detail.unavailable")}
              </Text>
            </DashboardCard>
          ) : (
            <DashboardCard>
              <Text style={[styles.body, { color: theme.mutedForeground }]}>
                {t("agentSessions.detail.notFound")}
              </Text>
            </DashboardCard>
          )}
        </DashboardScrollView>
      )}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  content: { padding: gutter, gap: space.md },
  footer: { gap: space.md },
  row: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  body: { fontSize: fontSizes.sm, lineHeight: fontSizes.sm * 1.5 },
  meta: { fontSize: fontSizes.xs, marginTop: space.xs },
  phase: { fontSize: fontSizes.sm, fontWeight: fontWeights.medium },
});

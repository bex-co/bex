import { useMemo, useState } from "react";
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
import {
  useRecovery,
  useRecoveryEnvironment,
} from "@/common/hooks/use-recovery";
import { recoveryAvailable } from "@/common/hooks/recovery-coordinator";
import { formatTimestamp } from "@/common/format-util";
import {
  fontSizes,
  fontWeights,
  gutter,
  space,
  useTheme,
} from "@/common/theme";
import { MobileAgentSessionsDocument } from "@/generated-graphql";
import { orderSessions, sessionPhaseView, type SessionTone } from "./lifecycle";
import { SessionComposer } from "./composer/session-composer-screen";

export function SessionsListScreen() {
  const { t, language } = useTranslations();
  const theme = useTheme().colorTheme;
  const { selected } = useWorkspace();
  const recoveryEnvironment = useRecoveryEnvironment();
  const [composing, setComposing] = useState(false);
  const { data, loading, error, refetch, networkStatus } = useQuery(
    MobileAgentSessionsDocument,
    {
      variables: { ownerId: selected?.id ?? "" },
      skip: !selected,
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      notifyOnNetworkStatusChange: true,
      pollInterval: recoveryAvailable(recoveryEnvironment) ? 30_000 : 0,
    },
  );
  const recovery = useRecovery({ attempt: async () => void (await refetch()) });

  const sessions = useMemo(
    () => orderSessions((data?.agentSessions ?? []).filter((s) => s?.id)),
    [data],
  );

  const toneColor = (tone: SessionTone): string => {
    switch (tone) {
      case "active":
        return theme.primary;
      case "danger":
        return theme.warning;
      case "success":
        return theme.foreground;
      default:
        return theme.mutedForeground;
    }
  };

  const initialLoading = loading && !data;
  const refreshing = networkStatus === NetworkStatus.refetch;

  return (
    <SafeAreaView style={[styles.safe, { backgroundColor: theme.background }]}>
      <TopBar
        title={t("navigation.sessions")}
        right={
          <View style={styles.headerRight}>
            {loading && data ? (
              <ActivityIndicator color={theme.primary} />
            ) : null}
            <Pressable
              testID="agent-session-new"
              accessibilityRole="button"
              accessibilityLabel={t("agentSessions.composer.new")}
              onPress={() => setComposing(true)}
              hitSlop={8}
            >
              <Ionicons name="add" size={24} color={theme.primary} />
            </Pressable>
          </View>
        }
      />
      {composing ? (
        <SessionComposer onClose={() => setComposing(false)} />
      ) : null}
      <DashboardScrollView
        refreshing={refreshing}
        onRefresh={() => void recovery.manualRetry()}
        contentContainerStyle={styles.content}
      >
        {initialLoading ? (
          <DashboardCard>
            <ActivityIndicator color={theme.primary} />
          </DashboardCard>
        ) : error && !data ? (
          <DashboardCard>
            <Text
              accessibilityRole="alert"
              style={[styles.notice, { color: theme.mutedForeground }]}
            >
              {t("agentSessions.unavailable")}
            </Text>
          </DashboardCard>
        ) : sessions.length === 0 ? (
          <DashboardCard>
            <Text style={[styles.emptyTitle, { color: theme.foreground }]}>
              {t("agentSessions.emptyTitle")}
            </Text>
            <Text style={[styles.emptyBody, { color: theme.mutedForeground }]}>
              {t("agentSessions.emptyBody")}
            </Text>
          </DashboardCard>
        ) : (
          <DashboardCard>
            {sessions.map((session, index) => {
              const phase = sessionPhaseView(session.phase);
              return (
                <Pressable
                  key={session.id}
                  testID={`agent-session-row-${session.id}`}
                  accessibilityRole="button"
                  onPress={() => router.push(`/sessions/${session.id}`)}
                  style={[
                    styles.row,
                    index > 0 && {
                      borderTopColor: theme.border,
                      borderTopWidth: StyleSheet.hairlineWidth,
                    },
                  ]}
                >
                  <View style={styles.rowMain}>
                    <Text
                      numberOfLines={1}
                      style={[styles.repo, { color: theme.foreground }]}
                    >
                      {session.repo}
                    </Text>
                    <Text
                      numberOfLines={1}
                      style={[styles.branch, { color: theme.mutedForeground }]}
                    >
                      {session.branch}
                    </Text>
                    <Text
                      style={[styles.meta, { color: theme.mutedForeground }]}
                    >
                      {formatTimestamp(session.updatedAt, language)}
                      {session.prNumber ? ` · #${session.prNumber}` : ""}
                    </Text>
                  </View>
                  <View style={styles.rowRight}>
                    <Text
                      style={[styles.phase, { color: toneColor(phase.tone) }]}
                    >
                      {t(phase.labelKey)}
                    </Text>
                    <Ionicons
                      name="chevron-forward"
                      size={16}
                      color={theme.mutedForeground}
                    />
                  </View>
                </Pressable>
              );
            })}
          </DashboardCard>
        )}
      </DashboardScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  headerRight: { flexDirection: "row", alignItems: "center", gap: space.sm },
  content: { padding: gutter, gap: space.md },
  notice: { fontSize: fontSizes.sm, lineHeight: fontSizes.sm * 1.5 },
  emptyTitle: { fontSize: fontSizes.md, fontWeight: fontWeights.medium },
  emptyBody: {
    fontSize: fontSizes.sm,
    lineHeight: fontSizes.sm * 1.5,
    marginTop: space.xs,
  },
  row: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingVertical: space.md,
    gap: space.sm,
  },
  rowMain: { flex: 1, gap: 2 },
  repo: { fontSize: fontSizes.md, fontWeight: fontWeights.medium },
  branch: { fontSize: fontSizes.sm },
  meta: { fontSize: fontSizes.xs },
  rowRight: { flexDirection: "row", alignItems: "center", gap: space.xs },
  phase: { fontSize: fontSizes.xs, fontWeight: fontWeights.medium },
});

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
import { Button } from "@/components/button";
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
import { fontSizes, fontWeights, space, useTheme } from "@/common/theme";
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
        return theme.error;
      case "success":
        return theme.success;
      default:
        return theme.mutedForeground;
    }
  };

  const initialLoading = loading && !data;
  const refreshing = networkStatus === NetworkStatus.refetch;

  return (
    <SafeAreaView
      edges={["top", "left", "right"]}
      style={[styles.safe, { backgroundColor: theme.background }]}
    >
      <DashboardScrollView
        header={
          <TopBar
            right={
              <View style={styles.headerRight}>
                {loading && data ? (
                  <ActivityIndicator color={theme.primary} />
                ) : null}
                <Pressable
                  accessibilityRole="button"
                  testID="new-agent-session"
                  accessibilityLabel={t("agentSessions.composer.new")}
                  onPress={() => setComposing(true)}
                  hitSlop={4}
                  style={[
                    styles.newButton,
                    { backgroundColor: theme.primaryMuted },
                  ]}
                >
                  <Ionicons name="add" size={20} color={theme.primary} />
                  <Text style={[styles.newLabel, { color: theme.primary }]}>
                    {t("agentSessions.newShort")}
                  </Text>
                </Pressable>
              </View>
            }
          />
        }
        refreshing={refreshing}
        onRefresh={() => void recovery.manualRetry()}
        contentContainerStyle={styles.content}
      >
        <View style={styles.intro}>
          <Text
            accessibilityRole="header"
            style={[styles.introTitle, { color: theme.foreground }]}
          >
            {t("agentSessions.heading")}
          </Text>
          <Text style={[styles.emptyBody, { color: theme.mutedForeground }]}>
            {t("agentSessions.description")}
          </Text>
        </View>
        {error && data ? (
          <Text
            accessibilityRole="alert"
            style={[styles.notice, { color: theme.warning }]}
          >
            {t("agentSessions.refreshError")}
          </Text>
        ) : null}
        {initialLoading ? (
          <DashboardCard>
            <View style={styles.loading}>
              <ActivityIndicator
                accessibilityLabel={t("agentSessions.loading")}
                color={theme.primary}
              />
              <Text
                style={[styles.emptyBody, { color: theme.mutedForeground }]}
              >
                {t("agentSessions.loading")}
              </Text>
            </View>
          </DashboardCard>
        ) : error && !data ? (
          <DashboardCard>
            <Text
              accessibilityRole="alert"
              style={[styles.notice, { color: theme.mutedForeground }]}
            >
              {t("agentSessions.unavailable")}
            </Text>
            <Button
              type="outline"
              style={{ marginTop: space.lg }}
              onPress={() => void recovery.manualRetry()}
              loading={refreshing}
            >
              {t("auth.retry")}
            </Button>
          </DashboardCard>
        ) : sessions.length === 0 ? (
          <DashboardCard>
            <View
              style={[
                styles.emptyIcon,
                { backgroundColor: theme.primaryMuted },
              ]}
            >
              <Ionicons
                name="sparkles-outline"
                size={28}
                color={theme.primary}
              />
            </View>
            <Text style={[styles.emptyTitle, { color: theme.foreground }]}>
              {t("agentSessions.emptyTitle")}
            </Text>
            <Text style={[styles.emptyBody, { color: theme.mutedForeground }]}>
              {t("agentSessions.emptyBody")}
            </Text>
            <Button
              style={{ marginTop: space.xl }}
              onPress={() => setComposing(true)}
            >
              {t("agentSessions.composer.new")}
            </Button>
          </DashboardCard>
        ) : (
          <DashboardCard>
            {sessions.map((session, index) => {
              const phase = sessionPhaseView(session.phase);
              return (
                <Pressable
                  key={session.id}
                  accessibilityRole="button"
                  onPress={() => router.push(`/sessions/${session.id}`)}
                  style={({ pressed }) => [
                    styles.row,
                    { opacity: pressed ? 0.65 : 1 },
                    index > 0 && {
                      borderTopColor: theme.border,
                      borderTopWidth: StyleSheet.hairlineWidth,
                    },
                  ]}
                >
                  <View
                    style={[
                      styles.sessionIcon,
                      { backgroundColor: theme.primaryMuted },
                    ]}
                  >
                    <Ionicons
                      name="git-branch-outline"
                      size={20}
                      color={theme.primary}
                    />
                  </View>
                  <View style={styles.rowMain}>
                    <Text
                      numberOfLines={2}
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
      {composing ? (
        <SessionComposer onClose={() => setComposing(false)} />
      ) : null}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  headerRight: { flexDirection: "row", alignItems: "center", gap: space.sm },
  content: { gap: space.lg },
  intro: { gap: space.xs },
  introTitle: { fontSize: fontSizes.xl, fontWeight: fontWeights.semibold },
  newButton: {
    flexDirection: "row",
    alignItems: "center",
    gap: space.xs,
    minHeight: 44,
    paddingHorizontal: space.md,
    borderRadius: space.md,
  },
  newLabel: { fontSize: fontSizes.md, fontWeight: fontWeights.semibold },
  emptyIcon: {
    width: 56,
    height: 56,
    borderRadius: space.lg,
    alignItems: "center",
    justifyContent: "center",
    marginBottom: space.lg,
  },
  sessionIcon: {
    width: 36,
    height: 36,
    borderRadius: space.sm,
    alignItems: "center",
    justifyContent: "center",
  },
  loading: {
    minHeight: 140,
    justifyContent: "center",
    alignItems: "center",
    gap: space.sm,
  },
  notice: { fontSize: fontSizes.sm, lineHeight: fontSizes.sm * 1.5 },
  emptyTitle: { fontSize: fontSizes.xl, fontWeight: fontWeights.semibold },
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
  repo: { fontSize: fontSizes.lg, fontWeight: fontWeights.semibold },
  branch: { fontSize: fontSizes.sm },
  meta: { fontSize: fontSizes.xs },
  rowRight: {
    flexDirection: "row",
    alignItems: "center",
    gap: space.xs,
    maxWidth: "35%",
    flexShrink: 1,
  },
  phase: {
    flexShrink: 1,
    fontSize: fontSizes.sm,
    fontWeight: fontWeights.medium,
  },
});

import { useCallback, useMemo, useState, type ReactNode } from "react";
import {
  ActivityIndicator,
  FlatList,
  Pressable,
  RefreshControl,
  StyleSheet,
  Text,
  View,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useTheme } from "@/common/theme";
import { fontSizes, fontWeights, gutter, space } from "@/common/theme";
import { useThemeStyle } from "@/common/hooks/use-theme-style";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/workspace-provider";
import { workspaceStatusLabel } from "@/features/workspaces/workspace-status";
import type { ColorTheme } from "@/types/theme-props";
import type { MobileWorkspace } from "@/features/workspaces/workspace-selection";

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    listArea: { flex: 1 },
    listContent: { flexGrow: 1, paddingBottom: space.sm },
    sectionLabel: {
      fontSize: fontSizes.sm,
      fontWeight: fontWeights.medium,
      color: theme.mutedForeground,
      paddingHorizontal: gutter,
      paddingTop: space.lg,
      paddingBottom: space.sm,
    },
    row: {
      flexDirection: "row",
      alignItems: "center",
      gap: space.md,
      paddingHorizontal: gutter,
      paddingVertical: space.md,
      minHeight: 56,
    },
    rowSelected: { backgroundColor: theme.primaryMuted },
    rowPressed: { opacity: 0.7 },
    rowCopy: { flex: 1, minWidth: 0 },
    rowName: {
      fontSize: fontSizes.lg,
      fontWeight: fontWeights.medium,
      color: theme.foreground,
    },
    rowStatus: {
      marginTop: space.xxs,
      fontSize: fontSizes.sm,
      color: theme.mutedForeground,
    },
    state: { alignItems: "center", paddingVertical: space.xxl, gap: space.md },
    stateText: {
      fontSize: fontSizes.md,
      color: theme.mutedForeground,
      textAlign: "center",
      paddingHorizontal: gutter,
    },
    retry: {
      minHeight: 44,
      justifyContent: "center",
      paddingHorizontal: space.lg,
    },
    retryText: {
      color: theme.primary,
      fontWeight: fontWeights.medium,
      fontSize: fontSizes.md,
    },
    offline: {
      color: theme.mutedForeground,
      fontSize: fontSizes.sm,
      paddingHorizontal: gutter,
      paddingBottom: space.sm,
    },
  });

/**
 * The drawer's primary scrollable context list: every accessible workspace in
 * API order, the current one marked, switching routed only through the reviewed
 * `switchWorkspace` boundary. Selecting closes the drawer at a safe terminal
 * point (a no-op tap immediately; a real switch only after the boundary reset
 * resolves) so a switch never flashes prior-workspace data.
 */
export function WorkspaceList({ onSelected }: { onSelected: () => void }) {
  const styles = useThemeStyle(getStyles);
  const theme = useTheme().colorTheme;
  const { t } = useTranslations();
  const {
    status,
    workspaces,
    selected,
    switching,
    offline,
    switchWorkspace,
    retry,
  } = useWorkspace();
  const [refreshing, setRefreshing] = useState(false);

  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    try {
      await retry();
    } catch {
      // retry() surfaces its failure through the provider's error status; the
      // list simply stops the spinner.
    } finally {
      setRefreshing(false);
    }
  }, [retry]);

  const handleSelect = useCallback(
    (workspace: MobileWorkspace) => {
      if (switching) return;
      if (workspace.id === selected?.id) {
        onSelected();
        return;
      }
      switchWorkspace(workspace.id).then(onSelected, () => undefined);
    },
    [onSelected, selected?.id, switchWorkspace, switching],
  );

  const statusLabel = useCallback(
    (workspace: MobileWorkspace) =>
      workspaceStatusLabel(workspace, {
        role: (slug) => t(`drawer.roles.${slug}`),
        plan: (slug) => t(`drawer.plans.${slug}`),
      }),
    [t],
  );

  const header = useMemo(
    () => (
      <>
        <Text style={styles.sectionLabel}>{t("drawer.workspaces")}</Text>
        {offline ? (
          <Text style={styles.offline}>{t("workspace.offline")}</Text>
        ) : null}
      </>
    ),
    [offline, styles, t],
  );

  const renderItem = useCallback(
    ({ item }: { item: MobileWorkspace }) => {
      const isSelected = item.id === selected?.id;
      const rowStatus = statusLabel(item);
      return (
        <Pressable
          accessibilityRole="button"
          accessibilityState={{ selected: isSelected, disabled: switching }}
          accessibilityLabel={t("drawer.workspaceRow", {
            name: item.name,
            status: rowStatus || t("drawer.noStatus"),
          })}
          disabled={switching}
          onPress={() => handleSelect(item)}
          style={({ pressed }) => [
            styles.row,
            isSelected && styles.rowSelected,
            pressed && styles.rowPressed,
          ]}
        >
          <View style={styles.rowCopy}>
            <Text style={styles.rowName} numberOfLines={1}>
              {item.name}
            </Text>
            {rowStatus ? (
              <Text style={styles.rowStatus} numberOfLines={1}>
                {rowStatus}
              </Text>
            ) : null}
          </View>
          {isSelected ? (
            <Ionicons name="checkmark-circle" size={22} color={theme.primary} />
          ) : null}
        </Pressable>
      );
    },
    [
      handleSelect,
      selected?.id,
      statusLabel,
      styles,
      switching,
      t,
      theme.primary,
    ],
  );

  // Loading/error/empty share one framed panel; only the inner content differs.
  let stateContent: ReactNode = null;
  if (workspaces.length === 0) {
    if (status === "loading") {
      stateContent = (
        <>
          <ActivityIndicator color={theme.primary} />
          <Text style={styles.stateText}>{t("workspace.loadingBody")}</Text>
        </>
      );
    } else if (status === "error") {
      stateContent = (
        <>
          <Text style={styles.stateText}>
            {t(offline ? "workspace.offlineBody" : "workspace.errorBody")}
          </Text>
          <Pressable
            accessibilityRole="button"
            accessibilityLabel={t("auth.retry")}
            style={styles.retry}
            onPress={() => void handleRefresh()}
          >
            <Text style={styles.retryText}>{t("auth.retry")}</Text>
          </Pressable>
        </>
      );
    } else {
      stateContent = (
        <Text style={styles.stateText}>{t("workspace.emptyBody")}</Text>
      );
    }
  }

  if (stateContent) {
    return (
      <View style={styles.listArea}>
        {header}
        <View style={styles.state}>{stateContent}</View>
      </View>
    );
  }

  return (
    <FlatList
      style={styles.listArea}
      contentContainerStyle={styles.listContent}
      data={workspaces}
      keyExtractor={(workspace) => workspace.id}
      ListHeaderComponent={header}
      refreshControl={
        <RefreshControl
          refreshing={refreshing}
          onRefresh={() => void handleRefresh()}
          tintColor={theme.isDark ? "white" : "black"}
        />
      }
      renderItem={renderItem}
    />
  );
}

import { useMemo, useRef, useState } from "react";
import {
  FlatList,
  Pressable,
  StyleSheet,
  Text,
  TextInput,
  View,
  type ListRenderItemInfo,
} from "react-native";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  fonts,
  fontSizes,
  gutter,
  monoMinFontSize,
  space,
  useTheme,
} from "@/common/theme";
import { LogSession } from "./log-session";
import type { LogFilters, LogLine, LogType } from "./types";
import { useLogSession } from "./use-log-session";

const filterTypes: Array<LogType | "all"> = [
  "all",
  "app",
  "build",
  "predeploy",
  "request",
];

export function LogViewer({
  resource,
  session,
  initialType = "all",
}: {
  resource: string;
  session: LogSession;
  initialType?: LogType | "all";
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const list = useRef<FlatList<LogLine>>(null);
  const [selectedType, setSelectedType] = useState<LogType | "all">(
    initialType,
  );
  const [text, setText] = useState("");
  const filters = useMemo<LogFilters>(
    () => ({
      resource,
      types: selectedType === "all" ? undefined : [selectedType],
      text: text.trim() || undefined,
      direction: "backward",
      limit: 100,
    }),
    [resource, selectedType, text],
  );
  const state = useLogSession(session, filters);

  const renderLine = ({ item }: ListRenderItemInfo<LogLine>) => (
    <View style={[styles.line, { borderBottomColor: theme.border }]}>
      <Text style={[styles.time, { color: theme.mutedForeground }]}>
        {formatLogTime(item.timestamp)}
      </Text>
      <Text selectable style={[styles.message, { color: theme.foreground }]}>
        {item.message}
      </Text>
    </View>
  );

  return (
    <View style={[styles.root, { backgroundColor: theme.background }]}>
      <View style={styles.filters}>
        <TextInput
          accessibilityLabel={t("logs.searchLabel")}
          placeholder={t("logs.searchPlaceholder")}
          placeholderTextColor={theme.mutedForeground}
          value={text}
          onChangeText={setText}
          autoCapitalize="none"
          autoCorrect={false}
          style={[
            styles.search,
            {
              backgroundColor: theme.card,
              borderColor: theme.border,
              color: theme.foreground,
            },
          ]}
        />
        <FlatList
          horizontal
          showsHorizontalScrollIndicator={false}
          data={filterTypes}
          keyExtractor={(item) => item}
          contentContainerStyle={styles.typeRow}
          renderItem={({ item }) => {
            const selected = item === selectedType;
            return (
              <Pressable
                accessibilityRole="button"
                accessibilityState={{ selected }}
                accessibilityLabel={t(`logs.type.${item}`)}
                onPress={() => setSelectedType(item)}
                style={[
                  styles.typeChip,
                  {
                    backgroundColor: selected ? theme.primaryMuted : theme.card,
                    borderColor: selected ? theme.primary : theme.border,
                  },
                ]}
              >
                <Text
                  style={{ color: selected ? theme.primary : theme.foreground }}
                >
                  {t(`logs.type.${item}`)}
                </Text>
              </Pressable>
            );
          }}
        />
      </View>

      <View style={[styles.status, { borderColor: theme.border }]}>
        <Text style={{ color: theme.mutedForeground }}>
          {state.tailBlockedByStoreOnlyFilters
            ? t("logs.historyOnly")
            : t(`logs.phase.${state.phase}`)}
        </Text>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel={state.paused ? t("logs.follow") : t("logs.pause")}
          onPress={() => {
            session.setPaused(!state.paused);
            if (state.paused) list.current?.scrollToEnd({ animated: true });
          }}
          style={[styles.followButton, { borderColor: theme.border }]}
        >
          <Text style={{ color: theme.primary }}>
            {state.paused
              ? `${t("logs.follow")}${state.unseen > 0 ? ` (${state.unseen})` : ""}`
              : t("logs.pause")}
          </Text>
        </Pressable>
      </View>

      {state.error ? (
        <Text
          accessibilityRole="alert"
          style={[styles.error, { color: theme.error }]}
        >
          {state.error.code === "store_unavailable"
            ? t("logs.storeUnavailable")
            : t("logs.unavailable")}
        </Text>
      ) : null}

      <FlatList
        ref={list}
        data={state.lines as LogLine[]}
        keyExtractor={(item) => item.id}
        renderItem={renderLine}
        initialNumToRender={30}
        maxToRenderPerBatch={30}
        windowSize={9}
        removeClippedSubviews
        onContentSizeChange={() => {
          if (!state.paused) list.current?.scrollToEnd({ animated: false });
        }}
        ListEmptyComponent={
          !state.error && state.phase !== "catching_up" ? (
            <Text style={[styles.empty, { color: theme.mutedForeground }]}>
              {t("logs.empty")}
            </Text>
          ) : null
        }
      />
    </View>
  );
}

export function createLogViewerSession(
  transport: ConstructorParameters<typeof LogSession>[0],
): LogSession {
  return new LogSession(transport, 500);
}

export function formatLogTime(timestamp: string): string {
  const parsed = new Date(timestamp);
  if (Number.isNaN(parsed.getTime())) return timestamp;
  return parsed.toISOString().slice(11, 23);
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  filters: { paddingHorizontal: gutter, gap: space.sm },
  search: {
    minHeight: 44,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: space.sm,
    paddingHorizontal: space.md,
    fontSize: fontSizes.md,
  },
  typeRow: { gap: space.sm, paddingBottom: space.sm },
  typeChip: {
    minHeight: 36,
    justifyContent: "center",
    borderRadius: 18,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: space.md,
  },
  status: {
    minHeight: 44,
    marginHorizontal: gutter,
    borderTopWidth: StyleSheet.hairlineWidth,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  followButton: {
    minHeight: 36,
    justifyContent: "center",
    borderRadius: 18,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: space.md,
  },
  error: { paddingHorizontal: gutter, paddingVertical: space.sm },
  empty: { padding: gutter, textAlign: "center" },
  line: {
    flexDirection: "row",
    gap: space.md,
    paddingHorizontal: gutter,
    paddingVertical: space.sm,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  time: {
    width: 92,
    fontFamily: fonts.mono,
    fontSize: monoMinFontSize,
    lineHeight: 20,
  },
  message: {
    flex: 1,
    fontFamily: fonts.mono,
    fontSize: monoMinFontSize,
    lineHeight: 20,
  },
});

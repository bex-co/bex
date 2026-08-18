import { memo, useCallback, useRef, type ReactNode } from "react";
import {
  ActivityIndicator,
  FlatList,
  KeyboardAvoidingView,
  type NativeScrollEvent,
  type NativeSyntheticEvent,
  Platform,
  StyleSheet,
  Text,
  View,
} from "react-native";
import type { ChatStatus, UIMessage } from "ai";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  fontSizes,
  fontWeights,
  gutter,
  space,
  useTheme,
} from "@/common/theme";
import { AgentPartView } from "./part-view";
import type { PartLike } from "./parts";

const STREAM_END_THRESHOLD = 80;

export function AgentConversationList({
  messages,
  status,
  header,
  footer,
  notice,
  interaction,
  refreshing = false,
  onRefresh,
}: {
  messages: UIMessage[];
  status: ChatStatus;
  header?: ReactNode;
  footer?: ReactNode;
  notice?: ReactNode;
  interaction?: ReactNode;
  refreshing?: boolean;
  onRefresh?: () => void;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const list = useRef<FlatList<UIMessage>>(null);
  const pinnedToEnd = useRef(true);
  const streaming = status === "streaming" || status === "submitted";
  const renderItem = useCallback(
    ({ item }: { item: UIMessage }) => <ConversationMessage message={item} />,
    [],
  );
  return (
    <KeyboardAvoidingView
      behavior={Platform.OS === "ios" ? "padding" : undefined}
      keyboardVerticalOffset={60}
      style={styles.list}
    >
      <FlatList
        ref={list}
        testID="agent-session-conversation"
        data={messages}
        keyExtractor={(message) => message.id}
        renderItem={renderItem}
        keyboardDismissMode="on-drag"
        keyboardShouldPersistTaps="handled"
        initialNumToRender={16}
        maxToRenderPerBatch={10}
        windowSize={7}
        contentContainerStyle={styles.listContent}
        style={styles.list}
        refreshing={refreshing}
        onRefresh={onRefresh}
        scrollEventThrottle={100}
        onScroll={(event: NativeSyntheticEvent<NativeScrollEvent>) => {
          const { contentOffset, contentSize, layoutMeasurement } =
            event.nativeEvent;
          pinnedToEnd.current =
            contentSize.height - (contentOffset.y + layoutMeasurement.height) <=
            STREAM_END_THRESHOLD;
        }}
        ListHeaderComponent={
          <View style={styles.sectionStack}>
            {header}
            <Text style={[styles.sectionTitle, { color: theme.foreground }]}>
              {t("agentSessions.conversation.title")}
            </Text>
            {notice}
          </View>
        }
        ListEmptyComponent={<EmptyConversation streaming={streaming} />}
        ListFooterComponent={
          <View style={styles.sectionStack}>
            {streaming && messages.length > 0 ? <StreamingIndicator /> : null}
            {interaction}
            {footer}
          </View>
        }
        onContentSizeChange={() => {
          if (streaming && pinnedToEnd.current) {
            list.current?.scrollToEnd({ animated: true });
          }
        }}
      />
    </KeyboardAvoidingView>
  );
}

const ConversationMessage = memo(function ConversationMessage({
  message,
}: {
  message: UIMessage;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const user = message.role === "user";
  return (
    <View
      accessibilityLabel={
        user
          ? t("agentSessions.conversation.userMessage")
          : t("agentSessions.conversation.agentMessage")
      }
      style={[styles.message, user && styles.userMessage]}
    >
      <Text
        style={[
          styles.role,
          { color: user ? theme.primary : theme.mutedForeground },
        ]}
      >
        {user
          ? t("agentSessions.conversation.you")
          : t("agentSessions.conversation.agent")}
      </Text>
      <View
        style={[
          styles.parts,
          user && {
            backgroundColor: theme.primaryMuted,
            borderColor: theme.primary,
          },
        ]}
      >
        {message.parts.map((part, index) => (
          <AgentPartView
            key={`${message.id}-${part.type}-${index}`}
            part={part as PartLike}
          />
        ))}
      </View>
    </View>
  );
});

function EmptyConversation({ streaming }: { streaming: boolean }) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  return (
    <View style={styles.empty}>
      {streaming ? <ActivityIndicator color={theme.primary} /> : null}
      <Text style={[styles.emptyText, { color: theme.mutedForeground }]}>
        {streaming
          ? t("agentSessions.conversation.connecting")
          : t("agentSessions.conversation.empty")}
      </Text>
    </View>
  );
}

function StreamingIndicator() {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  return (
    <View style={styles.streaming} accessibilityLiveRegion="polite">
      <ActivityIndicator size="small" color={theme.primary} />
      <Text style={[styles.emptyText, { color: theme.mutedForeground }]}>
        {t("agentSessions.conversation.streaming")}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  list: { flex: 1 },
  listContent: { padding: gutter, gap: space.lg, flexGrow: 1 },
  sectionStack: { gap: space.md },
  sectionTitle: { fontSize: fontSizes.xl, fontWeight: fontWeights.medium },
  message: { alignSelf: "stretch", gap: space.xs },
  userMessage: { alignSelf: "flex-end", width: "88%" },
  role: { fontSize: fontSizes.xs, fontWeight: fontWeights.medium },
  parts: {
    gap: space.sm,
    borderRadius: 12,
    borderWidth: 0,
    padding: space.xs,
  },
  empty: {
    flex: 1,
    minHeight: 220,
    alignItems: "center",
    justifyContent: "center",
    gap: space.sm,
  },
  emptyText: { fontSize: fontSizes.sm, textAlign: "center" },
  streaming: {
    minHeight: 44,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: space.xs,
  },
});

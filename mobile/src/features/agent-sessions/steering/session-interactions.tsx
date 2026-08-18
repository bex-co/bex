import { useMemo, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { randomUUID } from "expo-crypto";
import type { ChatStatus } from "ai";
import { Pressable, StyleSheet, Text, TextInput, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import type { RecoveryPhase } from "@/common/hooks/recovery-coordinator";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  fontSizes,
  fontWeights,
  rowMinHeight,
  space,
  useTheme,
} from "@/common/theme";
import {
  MobileAgentSessionDecisionsDocument,
  MobileRespondAgentSessionDecisionDocument,
} from "@/generated-graphql";
import { DecisionPanel } from "./decision-panel";

type ChatInteraction = {
  status: ChatStatus;
  error?: Error;
  available: boolean;
  recoveryPhase: RecoveryPhase;
  sendMessage: (message: { text: string; messageId: string }) => Promise<void>;
};

export function SessionInteractions({
  sessionId,
  chat,
}: {
  sessionId: string;
  chat: ChatInteraction;
}) {
  const decisions = useQuery(MobileAgentSessionDecisionsDocument, {
    variables: { id: sessionId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: chat.available ? 5_000 : 0,
  });
  const [respond] = useMutation(MobileRespondAgentSessionDecisionDocument);
  const [responding, setResponding] = useState(false);
  const [responseError, setResponseError] = useState(false);
  const pending = useMemo(
    () =>
      (decisions.data?.agentSessionDecisions ?? []).find(
        (decision) => decision.status === "pending",
      ) ?? null,
    [decisions.data],
  );

  if (pending) {
    return (
      <DecisionPanel
        decision={pending}
        submitting={responding}
        error={responseError}
        onRespond={async (action, value = {}) => {
          if (responding) return;
          setResponding(true);
          setResponseError(false);
          try {
            await respond({
              variables: {
                id: sessionId,
                decisionId: pending.id,
                version: pending.version,
                action,
                valueJson: Object.keys(value).length
                  ? JSON.stringify(value)
                  : null,
              },
            });
            await decisions.refetch();
          } catch {
            setResponseError(true);
            await decisions.refetch().catch(() => undefined);
          } finally {
            setResponding(false);
          }
        }}
      />
    );
  }

  return <TurnComposer chat={chat} />;
}

function TurnComposer({ chat }: { chat: ChatInteraction }) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const [prompt, setPrompt] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const busy =
    submitting || chat.status === "submitted" || chat.status === "streaming";
  const enabled = chat.available && chat.status === "ready" && !busy;

  const send = async () => {
    const text = prompt.trim();
    if (!text || !enabled) return;
    const messageId = randomUUID();
    setSubmitting(true);
    setUncertain(false);
    try {
      await chat.sendMessage({ text, messageId });
      setPrompt("");
    } catch {
      // Never retry an authorization/conflict/unknown-commit response. Keep the
      // text visible and require durable refresh/reconciliation first.
      setUncertain(true);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <View
      testID="agent-session-turn-composer"
      style={[
        styles.composer,
        { backgroundColor: theme.card, borderColor: theme.border },
      ]}
    >
      <Text style={[styles.composerTitle, { color: theme.foreground }]}>
        {t("agentSessions.steering.title")}
      </Text>
      <View style={styles.composeRow}>
        <TextInput
          testID="agent-session-turn-input"
          value={prompt}
          onChangeText={setPrompt}
          editable={!busy}
          multiline
          maxLength={10_000}
          placeholder={t("agentSessions.steering.placeholder")}
          placeholderTextColor={theme.mutedForeground}
          style={[
            styles.input,
            {
              color: theme.foreground,
              borderColor: theme.border,
              backgroundColor: theme.background,
            },
          ]}
        />
        <Pressable
          testID="agent-session-turn-send"
          accessibilityRole="button"
          accessibilityLabel={t("agentSessions.steering.send")}
          accessibilityState={{ disabled: !enabled || !prompt.trim() }}
          disabled={!enabled || !prompt.trim()}
          onPress={() => void send()}
          style={[
            styles.send,
            {
              backgroundColor:
                enabled && prompt.trim() ? theme.primary : theme.border,
            },
          ]}
        >
          <Ionicons name="arrow-up" size={20} color={theme.white} />
        </Pressable>
      </View>
      {!chat.available ? (
        <Text style={[styles.help, { color: theme.mutedForeground }]}>
          {t("agentSessions.steering.offline")}
        </Text>
      ) : chat.recoveryPhase !== "idle" ? (
        <Text style={[styles.help, { color: theme.mutedForeground }]}>
          {t("agentSessions.steering.reconnecting")}
        </Text>
      ) : busy ? (
        <Text style={[styles.help, { color: theme.mutedForeground }]}>
          {t("agentSessions.steering.pending")}
        </Text>
      ) : null}
      {uncertain || chat.error ? (
        <Text
          accessibilityRole="alert"
          style={[styles.help, { color: theme.warning }]}
        >
          {t("agentSessions.steering.unknown")}
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  composer: {
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: space.md,
    padding: space.md,
    gap: space.sm,
  },
  composerTitle: {
    fontSize: fontSizes.sm,
    fontWeight: fontWeights.medium,
  },
  composeRow: { flexDirection: "row", alignItems: "flex-end", gap: space.sm },
  input: {
    flex: 1,
    minHeight: rowMinHeight,
    maxHeight: 120,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: space.sm,
    paddingHorizontal: space.sm,
    paddingVertical: space.sm,
    fontSize: fontSizes.md,
    textAlignVertical: "top",
  },
  send: {
    width: rowMinHeight,
    height: rowMinHeight,
    borderRadius: rowMinHeight / 2,
    alignItems: "center",
    justifyContent: "center",
  },
  help: { fontSize: fontSizes.xs, lineHeight: fontSizes.xs * 1.5 },
});

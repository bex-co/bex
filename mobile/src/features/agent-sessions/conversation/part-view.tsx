import { useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useTranslations } from "@/common/hooks/use-translations";
import { fontSizes, fontWeights, fonts, space, useTheme } from "@/common/theme";
import {
  COLLAPSED_PART_CHARS,
  classifyConversationPart,
  type PartLike,
  type RenderablePart,
} from "./parts";
import { SafeMarkdownText } from "./safe-markdown-text";

export function AgentPartView({ part }: { part: PartLike }) {
  const rendered = classifyConversationPart(part);
  switch (rendered.kind) {
    case "text":
      return (
        <SafeMarkdownText value={rendered.text} style={styles.proseText} />
      );
    case "plan":
      return <PlanPart entries={rendered.entries} />;
    case "reasoning":
    case "diff":
    case "terminal":
    case "tool":
      return <ExpandablePart part={rendered} />;
    case "unknown":
      return <UnknownPart type={rendered.type} />;
  }
}

function PlanPart({
  entries,
}: {
  entries: Extract<RenderablePart, { kind: "plan" }>["entries"];
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  return (
    <View
      style={[styles.activity, { borderColor: theme.border }]}
      accessibilityLabel={t("agentSessions.conversation.plan")}
    >
      <Text style={[styles.activityTitle, { color: theme.foreground }]}>
        {t("agentSessions.conversation.plan")}
      </Text>
      {entries.length === 0 ? (
        <Text style={[styles.meta, { color: theme.mutedForeground }]}>—</Text>
      ) : (
        entries.map((entry, index) => {
          const complete = entry.status === "completed";
          return (
            <View key={`${entry.content}-${index}`} style={styles.planRow}>
              <Ionicons
                name={complete ? "checkmark-circle" : "ellipse-outline"}
                size={17}
                color={complete ? theme.success : theme.mutedForeground}
              />
              <Text style={[styles.planText, { color: theme.foreground }]}>
                {entry.content}
              </Text>
            </View>
          );
        })
      )}
    </View>
  );
}

function ExpandablePart({
  part,
}: {
  part: Exclude<RenderablePart, { kind: "text" | "plan" | "unknown" }>;
}) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  const [expanded, setExpanded] = useState(false);
  const { title, icon, body, code } = describePart(part, t);
  const shown =
    !expanded && body.length > COLLAPSED_PART_CHARS
      ? `${body.slice(0, COLLAPSED_PART_CHARS)}\n[…]`
      : body;
  return (
    <View style={[styles.activity, { borderColor: theme.border }]}>
      <Pressable
        accessibilityRole="button"
        accessibilityState={{ expanded }}
        accessibilityLabel={title}
        onPress={() => setExpanded((value) => !value)}
        style={styles.activityHeader}
      >
        <Ionicons name={icon} size={17} color={theme.primary} />
        <Text
          numberOfLines={1}
          style={[styles.activityTitle, { color: theme.foreground }]}
        >
          {title}
        </Text>
        <Ionicons
          name={expanded ? "chevron-up" : "chevron-down"}
          size={16}
          color={theme.mutedForeground}
        />
      </Pressable>
      {expanded || body.length <= COLLAPSED_PART_CHARS ? (
        code ? (
          <ScrollView horizontal showsHorizontalScrollIndicator>
            <Text selectable style={[styles.code, { color: theme.foreground }]}>
              {shown || t("agentSessions.conversation.noOutput")}
            </Text>
          </ScrollView>
        ) : (
          <SafeMarkdownText value={shown} style={styles.proseText} />
        )
      ) : (
        <Text
          numberOfLines={3}
          style={[styles.preview, { color: theme.mutedForeground }]}
        >
          {shown}
        </Text>
      )}
    </View>
  );
}

function UnknownPart({ type }: { type: string }) {
  const { t } = useTranslations();
  const theme = useTheme().colorTheme;
  return (
    <View style={[styles.unknown, { borderColor: theme.border }]}>
      <Ionicons
        name="extension-puzzle-outline"
        size={15}
        color={theme.mutedForeground}
      />
      <Text style={[styles.meta, { color: theme.mutedForeground }]}>
        {t("agentSessions.conversation.unknownPart", { type })}
      </Text>
    </View>
  );
}

type Translate = ReturnType<typeof useTranslations>["t"];

function describePart(
  part: Exclude<RenderablePart, { kind: "text" | "plan" | "unknown" }>,
  t: Translate,
): {
  title: string;
  icon: keyof typeof Ionicons.glyphMap;
  body: string;
  code: boolean;
} {
  switch (part.kind) {
    case "reasoning":
      return {
        title: t("agentSessions.conversation.reasoning"),
        icon: "bulb-outline",
        body: part.text,
        code: false,
      };
    case "diff":
      return {
        title: part.path || t("agentSessions.conversation.diff"),
        icon: "git-compare-outline",
        body: part.text,
        code: true,
      };
    case "terminal":
      return {
        title: t("agentSessions.conversation.terminal"),
        icon: "terminal-outline",
        body: part.text,
        code: true,
      };
    case "tool":
      return {
        title: part.name,
        icon:
          part.state === "output-error" ? "warning-outline" : "hammer-outline",
        body: [part.input, part.output, part.error]
          .filter(Boolean)
          .join("\n\n"),
        code: true,
      };
  }
}

const styles = StyleSheet.create({
  proseText: { fontSize: fontSizes.sm, lineHeight: fontSizes.sm * 1.55 },
  activity: {
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 10,
    padding: space.sm,
    gap: space.xs,
  },
  activityHeader: {
    minHeight: 40,
    flexDirection: "row",
    alignItems: "center",
    gap: space.xs,
  },
  activityTitle: {
    flex: 1,
    fontSize: fontSizes.sm,
    fontWeight: fontWeights.medium,
  },
  planRow: { flexDirection: "row", alignItems: "flex-start", gap: space.xs },
  planText: {
    flex: 1,
    fontSize: fontSizes.sm,
    lineHeight: fontSizes.sm * 1.45,
  },
  preview: { fontFamily: fonts.mono, fontSize: fontSizes.xs, lineHeight: 18 },
  code: {
    fontFamily: fonts.mono,
    fontSize: fontSizes.xs,
    lineHeight: 18,
    paddingBottom: space.xs,
  },
  unknown: {
    minHeight: 36,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 8,
    paddingHorizontal: space.sm,
    flexDirection: "row",
    alignItems: "center",
    gap: space.xs,
  },
  meta: { fontSize: fontSizes.xs },
});

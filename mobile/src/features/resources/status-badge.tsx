import { StyleSheet, Text, View } from "react-native";
import { humanizeToken } from "@/common/format-util";
import { fontSizes, fontWeights, space, useTheme } from "@/common/theme";
import { statusToneColor } from "./resource-groups";

/**
 * Dot + label for a resource status: one place owns the tone color and the
 * enum-token presentation so screens can't show a raw token or drift on
 * styling. `compact` is the list-row variant; the default suits detail cards.
 */
export function StatusBadge({
  status,
  compact = false,
}: {
  status: string;
  compact?: boolean;
}) {
  const theme = useTheme().colorTheme;
  const color = statusToneColor(status, theme);
  return (
    <View style={[styles.row, compact ? styles.rowCompact : null]}>
      <View
        style={[
          compact ? styles.dotCompact : styles.dot,
          { backgroundColor: color },
        ]}
      />
      <Text
        numberOfLines={1}
        style={[compact ? styles.textCompact : styles.text, { color }]}
      >
        {humanizeToken(status)}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "center", gap: space.sm },
  rowCompact: { gap: space.xs, flexShrink: 1 },
  dot: { width: 10, height: 10, borderRadius: 5 },
  dotCompact: { width: 8, height: 8, borderRadius: 4 },
  text: { fontSize: fontSizes.lg, fontWeight: fontWeights.medium },
  textCompact: { fontSize: fontSizes.sm, flexShrink: 1 },
});

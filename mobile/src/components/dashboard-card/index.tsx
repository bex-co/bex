import { ReactNode } from "react";
import {
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
  ViewStyle,
  useWindowDimensions,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { ColorTheme } from "@/types/theme-props";
import { useThemeStyle } from "@/common/hooks/use-theme-style";
import {
  fontSizes,
  fontWeights,
  gutter,
  space,
  useTheme,
} from "@/common/theme";
import { useTranslations } from "@/common/hooks/use-translations";

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    card: {
      backgroundColor: theme.card,
      borderRadius: space.lg,
      borderWidth: StyleSheet.hairlineWidth,
      borderColor: theme.border,
      paddingVertical: space.lg,
      overflow: "hidden",
    },
    header: {
      flexDirection: "row",
      alignItems: "center",
      flexWrap: "wrap",
      gap: space.sm,
      paddingHorizontal: gutter,
      marginBottom: space.md,
    },
    title: {
      flex: 1,
      fontSize: fontSizes.lg,
      fontWeight: fontWeights.semibold,
      color: theme.text01,
    },
    seeAll: {
      flexDirection: "row",
      alignItems: "center",
      minHeight: 44,
    },
    seeAllText: {
      fontSize: fontSizes.md,
      fontWeight: fontWeights.medium,
      color: theme.primary,
      marginRight: space.xxs,
    },
    content: {
      paddingHorizontal: gutter,
    },
  });

type DashboardCardProps = {
  /** Optional header title rendered on the left of the header row. */
  title?: string;
  /** When set, renders a right-aligned "see all →" affordance. */
  onSeeAll?: () => void;
  /** Optional actions slot rendered before the "see all" affordance. */
  right?: ReactNode;
  /**
   * When true, children render edge-to-edge (no horizontal content padding).
   * Use for full-width charts or rows that already manage their own insets.
   */
  bleed?: boolean;
  style?: ViewStyle;
  children: ReactNode;
};

/**
 * Rounded dashboard card with an optional header row
 * (title, actions slot, and a "see all →" affordance). Colors come from
 * theme tokens so it reads correctly in light and dark.
 */
export function DashboardCard({
  title,
  onSeeAll,
  right,
  bleed = false,
  style,
  children,
}: DashboardCardProps): React.ReactElement {
  const styles = useThemeStyle(getStyles);
  const theme = useTheme().colorTheme;
  const { t } = useTranslations();
  const { width, fontScale } = useWindowDimensions();
  const stackHeader =
    (width < 360 || fontScale > 1.3) && Boolean(right || onSeeAll);
  const hasHeader = Boolean(title || onSeeAll || right);

  return (
    <View style={[styles.card, style]}>
      {hasHeader && (
        <View
          style={[
            styles.header,
            stackHeader && { flexDirection: "column", alignItems: "stretch" },
          ]}
        >
          {title ? (
            <Text style={styles.title}>{title}</Text>
          ) : (
            <View style={{ flex: 1 }} />
          )}
          {right}
          {onSeeAll && (
            <TouchableOpacity
              style={styles.seeAll}
              onPress={onSeeAll}
              accessibilityRole="button"
              accessibilityLabel={t("common.seeAll")}
            >
              <Text style={styles.seeAllText}>{t("common.seeAll")}</Text>
              <Ionicons
                name="chevron-forward"
                size={16}
                color={theme.primary}
              />
            </TouchableOpacity>
          )}
        </View>
      )}
      <View style={bleed ? undefined : styles.content}>{children}</View>
    </View>
  );
}

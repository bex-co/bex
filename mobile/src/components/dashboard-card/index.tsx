import { ReactNode } from "react";
import {
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
  ViewStyle,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { ColorTheme } from "@/types/theme-props";
import { useThemeStyle } from "@/common/hooks/use-theme-style";
import {
  fontSizes,
  fontWeights,
  gutter,
  maxFontSizeMultipliers,
  space,
  useTheme,
} from "@/common/theme";
import { useTranslations } from "@/common/hooks/use-translations";

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    card: {
      backgroundColor: theme.black10,
      borderRadius: gutter,
      borderWidth: StyleSheet.hairlineWidth,
      borderColor: theme.black20,
      paddingVertical: space.lg,
      marginBottom: space.lg,
      overflow: "hidden",
    },
    header: {
      flexDirection: "row",
      alignItems: "center",
      paddingHorizontal: gutter,
      marginBottom: space.md,
    },
    title: {
      flex: 1,
      fontSize: fontSizes.xl,
      fontWeight: fontWeights.medium,
      color: theme.text01,
    },
    seeAll: {
      flexDirection: "row",
      alignItems: "center",
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
 * Monarch-style rounded dashboard card with an optional header row
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
  const hasHeader = Boolean(title || onSeeAll || right);

  return (
    <View style={[styles.card, style]}>
      {hasHeader && (
        <View style={styles.header}>
          {title ? (
            <Text
              maxFontSizeMultiplier={maxFontSizeMultipliers.heading}
              style={styles.title}
            >
              {title}
            </Text>
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
              <Text
                maxFontSizeMultiplier={maxFontSizeMultipliers.control}
                style={styles.seeAllText}
              >
                {t("common.seeAll")}
              </Text>
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

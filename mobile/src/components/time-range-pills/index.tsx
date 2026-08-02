import { StyleSheet, Text, TouchableOpacity, View } from "react-native";
import { ColorTheme } from "@/types/theme-props";
import { fontSizes, fontWeights, space } from "@/common/theme";
import { useThemeStyle } from "@/common/hooks/use-theme-style";

export type PillOption<T extends string> = {
  key: T;
  label: string;
};

type TimeRangePillsProps<T extends string> = {
  value: T;
  options: PillOption<T>[];
  onChange: (key: T) => void;
};

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    row: {
      flexDirection: "row",
      justifyContent: "center",
      alignItems: "center",
      paddingTop: space.sm,
    },
    // Pill internals are a self-contained segmented control; these compact
    // metrics are intentional and not part of the shared spacing scale.
    pill: {
      paddingHorizontal: 10,
      paddingVertical: 6,
      borderRadius: 14,
      marginHorizontal: 3,
    },
    pillActive: {
      backgroundColor: theme.primary,
    },
    label: {
      fontSize: fontSizes.sm,
      fontWeight: fontWeights.medium,
      color: theme.black80,
    },
    labelActive: {
      color: theme.white,
    },
  });

/**
 * Robinhood-style row of time-range pills (a small segmented control). Generic
 * over the option key so it can drive any chart's range selection.
 */
export function TimeRangePills<T extends string>({
  value,
  options,
  onChange,
}: TimeRangePillsProps<T>): React.ReactElement {
  const styles = useThemeStyle(getStyles);
  return (
    <View style={styles.row}>
      {options.map((option) => {
        const active = option.key === value;
        return (
          <TouchableOpacity
            key={option.key}
            style={[styles.pill, active && styles.pillActive]}
            onPress={() => onChange(option.key)}
            accessibilityRole="button"
            accessibilityState={{ selected: active }}
            accessibilityLabel={option.label}
          >
            <Text style={[styles.label, active && styles.labelActive]}>
              {option.label}
            </Text>
          </TouchableOpacity>
        );
      })}
    </View>
  );
}

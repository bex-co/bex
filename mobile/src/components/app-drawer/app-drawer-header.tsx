import { StyleSheet, TouchableOpacity } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useTheme } from "@/common/theme";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAppDrawer } from "./app-drawer-context";

/**
 * The menu trigger that opens the shared drawer. A 44pt hit target with the
 * hamburger icon; usable by button alone (the left-edge swipe is the gesture
 * alternative). Screens host it in their own header/title row. Must be rendered
 * inside an {@link AppDrawerProvider}.
 */
export function AppDrawerButton() {
  const theme = useTheme().colorTheme;
  const { t } = useTranslations();
  const { openDrawer } = useAppDrawer();
  return (
    <TouchableOpacity
      testID="app-drawer-button"
      accessibilityRole="button"
      accessibilityLabel={t("drawer.openMenu")}
      accessibilityHint={t("drawer.openMenuHint")}
      activeOpacity={0.7}
      onPress={openDrawer}
      style={styles.trigger}
    >
      <Ionicons name="menu" size={26} color={theme.foreground} />
    </TouchableOpacity>
  );
}

const styles = StyleSheet.create({
  trigger: {
    width: 44,
    height: 44,
    flexShrink: 0,
    alignItems: "center",
    justifyContent: "center",
  },
});

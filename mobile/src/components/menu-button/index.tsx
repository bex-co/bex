import { useRef, useState, type ReactNode } from "react";
import {
  Dimensions,
  Modal,
  Pressable,
  StyleSheet,
  Text,
  View,
} from "react-native";
import { fontSizes } from "@/common/theme";
import { useThemeStyle } from "@/common/hooks/use-theme-style";
import { ColorTheme } from "@/types/theme-props";

export type MenuButtonItem = {
  label: string;
  /** Trailing glyph, drawn at the row's right edge. */
  icon?: ReactNode;
  onPress: () => void;
};

type MenuButtonProps = {
  /** The trigger glyph; the button around it is a 44pt hit target. */
  icon: ReactNode;
  accessibilityLabel: string;
  items: MenuButtonItem[];
  onOpen?: () => void;
  testID?: string;
};

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    trigger: {
      alignItems: "center",
      justifyContent: "center",
      // No negative margin: the icon sits at the header's 16px gutter like every
      // other header action, so the glyph
      // stays symmetric with the left-side icons and lines up across tabs.
    },
    triggerPressed: {
      opacity: 0.6,
    },
    backdrop: {
      flex: 1,
    },
    menu: {
      position: "absolute",
      minWidth: 240,
      borderRadius: 14,
      backgroundColor: theme.white,
      // Shadows read as nothing on a dark surface, so a hairline border does
      // the edge definition in both themes.
      borderWidth: StyleSheet.hairlineWidth,
      borderColor: theme.black20,
      shadowColor: theme.black,
      shadowOffset: { width: 0, height: 2 },
      shadowOpacity: 0.15,
      shadowRadius: 8,
      elevation: 4,
      overflow: "hidden",
    },
    item: {
      flexDirection: "row",
      alignItems: "center",
      gap: 16,
      paddingVertical: 14,
      paddingHorizontal: 16,
    },
    itemPressed: {
      backgroundColor: theme.black10,
    },
    itemDivider: {
      borderTopWidth: StyleSheet.hairlineWidth,
      borderTopColor: theme.black20,
    },
    itemLabel: {
      flex: 1,
      fontSize: fontSizes.lg,
      color: theme.black,
    },
  });

/**
 * An icon button that drops a right-aligned menu beneath itself.
 *
 * Built for the header action slot: the popover is measured off the trigger in
 * window coordinates, so it stays pinned to the button on any screen width.
 */
export const MenuButton = ({
  icon,
  accessibilityLabel,
  items,
  onOpen,
  testID,
}: MenuButtonProps): React.ReactElement => {
  const styles = useThemeStyle(getStyles);
  const anchorRef = useRef<View>(null);
  const [visible, setVisible] = useState(false);
  const [menuPos, setMenuPos] = useState({ top: 0, right: 0 });

  const openMenu = () => {
    onOpen?.();
    anchorRef.current?.measureInWindow((x, y, width, height) => {
      const screenWidth = Dimensions.get("window").width;
      setMenuPos({ top: y + height + 4, right: screenWidth - (x + width) });
      setVisible(true);
    });
  };

  return (
    <View ref={anchorRef} collapsable={false}>
      <Pressable
        testID={testID}
        accessibilityRole="button"
        accessibilityLabel={accessibilityLabel}
        style={({ pressed }) => [
          styles.trigger,
          pressed && styles.triggerPressed,
        ]}
        onPress={openMenu}
        hitSlop={8}
      >
        {icon}
      </Pressable>

      <Modal
        visible={visible}
        transparent
        animationType="none"
        onRequestClose={() => setVisible(false)}
      >
        <Pressable style={styles.backdrop} onPress={() => setVisible(false)}>
          <View
            style={[styles.menu, { top: menuPos.top, right: menuPos.right }]}
          >
            {items.map((item, i) => (
              <Pressable
                key={item.label}
                accessibilityRole="button"
                style={({ pressed }) => [
                  styles.item,
                  i > 0 && styles.itemDivider,
                  pressed && styles.itemPressed,
                ]}
                onPress={() => {
                  setVisible(false);
                  item.onPress();
                }}
              >
                <Text style={styles.itemLabel}>{item.label}</Text>
                {item.icon}
              </Pressable>
            ))}
          </View>
        </Pressable>
      </Modal>
    </View>
  );
};

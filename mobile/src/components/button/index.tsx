import React, { useCallback, useMemo } from "react";
import {
  Pressable,
  Text,
  StyleSheet,
  PressableStateCallbackType,
  StyleProp,
  ViewStyle,
  ActivityIndicator,
} from "react-native";
import { useThemeStyle } from "@/common/hooks/use-theme-style";
import { ColorTheme } from "@/types/theme-props";
import { useTheme } from "@/common/theme";

type ButtonType = "primary" | "outline";

type ButtonProps = {
  type?: ButtonType;
  style?: StyleProp<ViewStyle>;
  children: React.ReactNode;
  loading?: boolean;
  onPress?: () => void;
  disabled?: boolean;
  accessibilityLabel?: string;
};

const getButtonStyles = (theme: ColorTheme) => {
  return StyleSheet.create({
    buttonBase: {
      minHeight: 48,
      borderRadius: 12,
      paddingVertical: 12,
      paddingHorizontal: 16,
      alignItems: "center",
      justifyContent: "center",
      flexDirection: "row",
    },
    buttonPrimary: {
      backgroundColor: theme.primary,
    },
    buttonPrimaryPressed: {
      opacity: 0.8,
    },
    buttonPrimaryText: {
      color: theme.onPrimary,
      fontSize: 16,
      fontWeight: "600",
      textAlign: "center",
      flexShrink: 1,
    },
    buttonOutline: {
      backgroundColor: "transparent",
      borderWidth: 1,
      borderColor: theme.primary,
    },
    buttonOutlinePressed: {
      opacity: 0.6,
    },
    buttonOutlineText: {
      color: theme.primary,
      fontSize: 16,
      textAlign: "center",
      flexShrink: 1,
    },
    buttonLoading: {
      marginRight: 8,
    },
  });
};

export const Button = (props: ButtonProps) => {
  const type = props.type || "primary";
  const styles = useThemeStyle(getButtonStyles);
  const pressableStyle = useCallback(
    ({ pressed }: PressableStateCallbackType) => {
      switch (type) {
        case "primary":
          return [
            styles.buttonBase,
            styles.buttonPrimary,
            pressed && styles.buttonPrimaryPressed,
            props.style,
            (props.disabled || !props.onPress) && { opacity: 0.45 },
          ];
        case "outline":
          return [
            styles.buttonBase,
            styles.buttonOutline,
            pressed && styles.buttonOutlinePressed,
            props.style,
            (props.disabled || !props.onPress) && { opacity: 0.45 },
          ];
      }
    },
    [styles, type, props.style, props.disabled, props.onPress],
  );

  const buttonTextStyle = useMemo(() => {
    switch (type) {
      case "primary":
        return styles.buttonPrimaryText;
      case "outline":
        return styles.buttonOutlineText;
    }
  }, [styles, type]);

  const theme = useTheme().colorTheme;

  return (
    <Pressable
      style={pressableStyle}
      onPress={props.onPress}
      disabled={props.disabled || props.loading || !props.onPress}
      accessibilityRole="button"
      accessibilityLabel={props.accessibilityLabel}
      accessibilityState={{
        disabled: props.disabled || props.loading || !props.onPress,
        busy: Boolean(props.loading),
      }}
    >
      {props.loading ? (
        <ActivityIndicator
          color={type === "primary" ? theme.onPrimary : theme.primary}
          style={styles.buttonLoading}
        />
      ) : null}
      <Text style={buttonTextStyle}>{props.children}</Text>
    </Pressable>
  );
};

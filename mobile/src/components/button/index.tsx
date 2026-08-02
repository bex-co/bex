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
};

const getButtonStyles = (theme: ColorTheme) => {
  return StyleSheet.create({
    buttonBase: {
      height: 44,
      borderRadius: 8,
      // flex: 1,
      alignItems: "center",
      justifyContent: "center",
      flexDirection: "row",
    },
    buttonPrimary: {
      backgroundColor: theme.primary,
    },
    buttonPrimaryPressed: {
      backgroundColor: theme.primaryDark,
    },
    buttonPrimaryText: {
      color: theme.white,
      fontSize: 16,
    },
    buttonOutline: {
      backgroundColor: theme.white,
      borderWidth: 1,
      borderColor: theme.primary,
    },
    buttonOutlinePressed: {
      opacity: 0.6,
    },
    buttonOutlineText: {
      color: theme.primary,
      fontSize: 16,
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
            props.style,
            styles.buttonBase,
            styles.buttonPrimary,
            pressed && styles.buttonPrimaryPressed,
          ];
        case "outline":
          return [
            props.style,
            styles.buttonBase,
            styles.buttonOutline,
            pressed && styles.buttonOutlinePressed,
          ];
      }
    },
    [styles, type, props.style],
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
      pointerEvents={props.onPress ? "auto" : "none"}
    >
      {props.loading ? (
        <ActivityIndicator
          color={type === "primary" ? theme.white : theme.primary}
          style={styles.buttonLoading}
        />
      ) : null}
      <Text style={buttonTextStyle}>{props.children}</Text>
    </Pressable>
  );
};

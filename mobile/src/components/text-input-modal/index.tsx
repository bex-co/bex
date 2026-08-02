import React, { useState, useEffect } from "react";
import {
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  Pressable,
  Modal,
  TextInput,
} from "react-native";
import Animated, {
  useSharedValue,
  useAnimatedStyle,
  withTiming,
} from "react-native-reanimated";
import { useTheme } from "@/common/theme";
import { ColorTheme } from "@/types/theme-props";

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    overlay: {
      flex: 1,
      backgroundColor: theme.overlay,
      justifyContent: "center",
      alignItems: "center",
      padding: 16,
    },
    modalContainer: {
      backgroundColor: theme.white,
      borderRadius: 16,
      width: "100%",
      maxWidth: 400,
    },
    header: {
      paddingHorizontal: 20,
      paddingTop: 20,
      paddingBottom: 12,
    },
    title: {
      fontSize: 18,
      fontWeight: "600",
      color: theme.text01,
      marginBottom: 8,
    },
    message: {
      fontSize: 14,
      color: theme.black80,
      lineHeight: 20,
    },
    inputContainer: {
      paddingHorizontal: 20,
      paddingBottom: 20,
    },
    input: {
      borderWidth: 1,
      borderColor: theme.black40,
      borderRadius: 8,
      paddingHorizontal: 12,
      paddingVertical: 10,
      fontSize: 16,
      color: theme.text01,
      backgroundColor: theme.white,
    },
    buttonContainer: {
      flexDirection: "row",
      borderTopWidth: StyleSheet.hairlineWidth,
      borderTopColor: theme.black40,
    },
    button: {
      flex: 1,
      paddingVertical: 14,
      alignItems: "center",
      justifyContent: "center",
    },
    buttonDivider: {
      width: StyleSheet.hairlineWidth,
      backgroundColor: theme.black40,
    },
    cancelButton: {
      color: theme.black80,
      fontSize: 16,
    },
    confirmButton: {
      color: theme.primary,
      fontSize: 16,
      fontWeight: "600",
    },
    confirmButtonDestructive: {
      color: theme.error,
      fontSize: 16,
      fontWeight: "600",
    },
    confirmButtonDisabled: {
      color: theme.black40,
      fontSize: 16,
      fontWeight: "600",
    },
  });

type TextInputModalProps = {
  visible: boolean;
  title: string;
  message?: string;
  placeholder?: string;
  onConfirm: (text: string) => void;
  onCancel: () => void;
  confirmButtonText?: string;
  cancelButtonText?: string;
  destructive?: boolean;
  validateInput?: (text: string) => boolean;
};

export const TextInputModal: React.FC<TextInputModalProps> = ({
  visible,
  title,
  message,
  placeholder,
  onConfirm,
  onCancel,
  confirmButtonText = "Confirm",
  cancelButtonText = "Cancel",
  destructive = false,
  validateInput,
}) => {
  const theme = useTheme().colorTheme;
  const styles = getStyles(theme);
  const [inputText, setInputText] = useState("");
  const opacity = useSharedValue(0);
  const scale = useSharedValue(0.9);

  useEffect(() => {
    if (visible) {
      setInputText("");
      opacity.value = withTiming(1, { duration: 200 });
      scale.value = withTiming(1, { duration: 200 });
    } else {
      opacity.value = withTiming(0, { duration: 150 });
      scale.value = withTiming(0.9, { duration: 150 });
    }
  }, [visible]);

  const overlayAnimatedStyle = useAnimatedStyle(() => ({
    opacity: opacity.value,
  }));

  const modalAnimatedStyle = useAnimatedStyle(() => ({
    transform: [{ scale: scale.value }],
  }));

  const handleConfirm = () => {
    if (validateInput && !validateInput(inputText)) {
      return;
    }
    onConfirm(inputText);
  };

  const isConfirmDisabled = validateInput && !validateInput(inputText);

  if (!visible) return null;

  return (
    <Modal
      visible={visible}
      transparent
      animationType="none"
      onRequestClose={onCancel}
    >
      <Animated.View style={[styles.overlay, overlayAnimatedStyle]}>
        <Pressable
          style={StyleSheet.absoluteFill}
          onPress={onCancel}
        ></Pressable>
        <Animated.View style={[styles.modalContainer, modalAnimatedStyle]}>
          <View style={styles.header}>
            <Text style={styles.title}>{title}</Text>
            {message && <Text style={styles.message}>{message}</Text>}
          </View>
          <View style={styles.inputContainer}>
            <TextInput
              style={styles.input}
              value={inputText}
              onChangeText={setInputText}
              placeholder={placeholder}
              placeholderTextColor={theme.black40}
              autoFocus
              autoCapitalize="none"
              autoCorrect={false}
            />
          </View>
          <View style={styles.buttonContainer}>
            <TouchableOpacity
              style={styles.button}
              onPress={onCancel}
              activeOpacity={0.6}
            >
              <Text style={styles.cancelButton}>{cancelButtonText}</Text>
            </TouchableOpacity>
            <View style={styles.buttonDivider} />
            <TouchableOpacity
              style={styles.button}
              onPress={handleConfirm}
              activeOpacity={0.6}
              disabled={isConfirmDisabled}
            >
              <Text
                style={
                  isConfirmDisabled
                    ? styles.confirmButtonDisabled
                    : destructive
                      ? styles.confirmButtonDestructive
                      : styles.confirmButton
                }
              >
                {confirmButtonText}
              </Text>
            </TouchableOpacity>
          </View>
        </Animated.View>
      </Animated.View>
    </Modal>
  );
};

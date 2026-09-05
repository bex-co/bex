import { ScreenToolbar } from "@/components/screen-toolbar";
import React, { useCallback, useEffect, useMemo, useRef } from "react";
import {
  Animated,
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  ScrollView,
  Pressable,
  Modal,
  useWindowDimensions,
} from "react-native";
import { headerActionStyle, useTheme } from "@/common/theme";
import { ColorTheme } from "@/types/theme-props";

const ITEM_HEIGHT = 50;
const VISIBLE_ITEMS = 5;
const WHEEL_HEIGHT = ITEM_HEIGHT * VISIBLE_ITEMS;

type PickerItem = {
  label: string;
  value: string;
  icon?: React.ReactNode;
};

type PickerProps = {
  visible: boolean;
  items: PickerItem[];
  onSelect: (item: PickerItem) => void;
  onCancel: () => void;
  selectedValue?: string;
  title?: string;
  confirmButtonText?: string;
  cancelButtonText?: string;
};

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    overlay: {
      flex: 1,
      backgroundColor: theme.overlay,
      justifyContent: "flex-end",
    },
    modalContainer: {
      backgroundColor: theme.white,
      borderTopLeftRadius: 16,
      borderTopRightRadius: 16,
      paddingBottom: 34, // Safe area for home indicator
    },
    headerAction: { minHeight: 44, justifyContent: "center" },
    cancelButton: {
      color: theme.black80,
      fontSize: 16,
    },
    doneButton: headerActionStyle(theme),
    wheelContainer: {
      height: WHEEL_HEIGHT,
      position: "relative",
      //   backgroundColor: "blue",
    },
    wheel: {
      height: WHEEL_HEIGHT,
      //   backgroundColor: "red",
    },
    wheelItem: {
      height: ITEM_HEIGHT,
      justifyContent: "center",
      alignItems: "center",
      flexDirection: "row",
      gap: 8,
    },
    wheelItemText: {
      fontSize: 18,
      color: theme.text01,
    },
    selectedItemText: {
      fontSize: 20,
      fontWeight: "600",
      color: theme.primary,
    },
    selectionIndicator: {
      position: "absolute",
      top: (WHEEL_HEIGHT - ITEM_HEIGHT) / 2,
      left: 0,
      right: 0,
      height: ITEM_HEIGHT,
      backgroundColor: theme.black10,
      borderRadius: 8,
      zIndex: -1,
    },
    fadeGradient: {
      position: "absolute",
      left: 0,
      right: 0,
      height: ITEM_HEIGHT * 2,
      zIndex: 1,
      opacity: 0.5,
      pointerEvents: "none",
    },
    fadeGradientTop: {
      top: 0,
      backgroundColor: theme.white,
    },
    fadeGradientBottom: {
      bottom: 0,
      backgroundColor: theme.white,
    },
    mask: {
      position: "absolute",
      top: 0,
      left: 0,
      right: 0,
      bottom: 0,
    },
  });

export const Picker: React.FC<PickerProps> = ({
  visible,
  items,
  onSelect,
  onCancel,
  selectedValue,
  title,
  confirmButtonText = "Done",
  cancelButtonText = "Cancel",
}) => {
  const theme = useTheme().colorTheme;
  const styles = getStyles(theme);
  const { height: screenHeight } = useWindowDimensions();

  const translateY = useRef(new Animated.Value(screenHeight)).current;
  const scrollY = useRef(0);
  const overlayOpacity = useRef(new Animated.Value(0)).current;

  const selectedIndex = useMemo(() => {
    if (!selectedValue) return 0;
    const index = items.findIndex((item) => item.value === selectedValue);
    return index >= 0 ? index : 0;
  }, [items, selectedValue]);

  const initialScrollY = selectedIndex * ITEM_HEIGHT;

  const showModal = useCallback(() => {
    translateY.stopAnimation();
    overlayOpacity.stopAnimation();
    translateY.setValue(screenHeight);
    overlayOpacity.setValue(0);
    Animated.parallel([
      Animated.timing(overlayOpacity, {
        toValue: 1,
        duration: 300,
        useNativeDriver: true,
      }),
      Animated.timing(translateY, {
        toValue: 0,
        duration: 300,
        useNativeDriver: true,
      }),
    ]).start();
  }, [overlayOpacity, screenHeight, translateY]);

  const hideModal = useCallback(() => {
    Animated.parallel([
      Animated.timing(overlayOpacity, {
        toValue: 0,
        duration: 300,
        useNativeDriver: true,
      }),
      Animated.timing(translateY, {
        toValue: screenHeight,
        duration: 300,
        useNativeDriver: true,
      }),
    ]).start(({ finished }) => {
      if (finished) onCancel();
    });
  }, [onCancel, overlayOpacity, screenHeight, translateY]);

  useEffect(() => {
    if (visible) {
      scrollY.current = initialScrollY;
      showModal();
    }
  }, [initialScrollY, showModal, visible]);

  const handleDone = useCallback(() => {
    if (items.length === 0) return;
    const currentIndex = Math.round(scrollY.current / ITEM_HEIGHT);
    const clampedIndex = Math.max(0, Math.min(currentIndex, items.length - 1));
    onSelect(items[clampedIndex]);
    hideModal();
  }, [items, onSelect, hideModal]);

  const handleCancel = useCallback(() => {
    hideModal();
  }, [hideModal]);

  const renderItem = useCallback(
    (item: PickerItem) => {
      const isSelected = item.value === selectedValue;
      return (
        <View
          key={item.value}
          testID={`picker-item-${item.value || "empty"}`}
          style={styles.wheelItem}
          accessible
          accessibilityRole="radio"
          accessibilityLabel={item.label}
          accessibilityState={{ selected: isSelected }}
          onAccessibilityTap={() => {
            onSelect(item);
            hideModal();
          }}
        >
          {item.icon}
          <Text
            style={[
              styles.wheelItemText,
              isSelected && styles.selectedItemText,
            ]}
          >
            {item.label}
          </Text>
        </View>
      );
    },
    [
      hideModal,
      onSelect,
      styles.wheelItemText,
      styles.selectedItemText,
      styles.wheelItem,
      selectedValue,
    ],
  );

  if (!visible) return null;

  return (
    <Modal
      visible={visible}
      transparent
      animationType="none"
      onRequestClose={handleCancel}
    >
      <Animated.View
        style={[styles.overlay, { opacity: overlayOpacity }]}
        accessibilityViewIsModal
      >
        <Pressable
          style={styles.mask}
          onPress={handleCancel}
          accessibilityRole="button"
          accessibilityLabel={cancelButtonText}
        />
        <Animated.View
          style={[styles.modalContainer, { transform: [{ translateY }] }]}
        >
          <ScreenToolbar
            title={title}
            textActions
            left={
              <TouchableOpacity
                testID="picker-cancel"
                style={styles.headerAction}
                onPress={handleCancel}
                accessibilityRole="button"
                accessibilityLabel={cancelButtonText}
              >
                <Text style={styles.cancelButton}>{cancelButtonText}</Text>
              </TouchableOpacity>
            }
            right={
              <TouchableOpacity
                testID="picker-confirm"
                style={styles.headerAction}
                onPress={handleDone}
                disabled={items.length === 0}
                accessibilityRole="button"
                accessibilityLabel={confirmButtonText}
                accessibilityState={{ disabled: items.length === 0 }}
              >
                <Text style={styles.doneButton}>{confirmButtonText}</Text>
              </TouchableOpacity>
            }
          />

          <View style={styles.wheelContainer}>
            <View style={styles.selectionIndicator} />

            <ScrollView
              style={styles.wheel}
              showsVerticalScrollIndicator={false}
              snapToInterval={ITEM_HEIGHT}
              decelerationRate="fast"
              onScroll={(event) => {
                scrollY.current = event.nativeEvent.contentOffset.y;
              }}
              scrollEventThrottle={16}
              contentOffset={{ x: 0, y: initialScrollY }}
            >
              {/* Add padding items to center the first and last items */}
              <View style={{ height: (WHEEL_HEIGHT - ITEM_HEIGHT) / 2 }} />
              {items.map(renderItem)}
              <View style={{ height: (WHEEL_HEIGHT - ITEM_HEIGHT) / 2 }} />
            </ScrollView>

            {/* Fade gradients for better UX */}
            <View style={[styles.fadeGradient, styles.fadeGradientTop]} />
            <View style={[styles.fadeGradient, styles.fadeGradientBottom]} />
          </View>
        </Animated.View>
      </Animated.View>
    </Modal>
  );
};

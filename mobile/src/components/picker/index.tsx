import React, { useCallback, useEffect, useMemo } from "react";
import {
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  ScrollView,
  Dimensions,
  Pressable,
  Modal,
} from "react-native";
import Animated, {
  useSharedValue,
  useAnimatedStyle,
  withTiming,
  runOnJS,
  useAnimatedScrollHandler,
  useAnimatedRef,
} from "react-native-reanimated";
import { headerActionStyle, useTheme } from "@/common/theme";
import { ColorTheme } from "@/types/theme-props";

const { height: screenHeight } = Dimensions.get("window");
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

const AnimatedScrollView = Animated.createAnimatedComponent(ScrollView);

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
    header: {
      flexDirection: "row",
      justifyContent: "space-between",
      alignItems: "center",
      paddingHorizontal: 16,
      paddingVertical: 16,
      borderBottomWidth: StyleSheet.hairlineWidth,
      borderBottomColor: theme.black40,
    },
    cancelButton: {
      color: theme.black80,
      fontSize: 16,
    },
    doneButton: headerActionStyle(theme),
    title: {
      fontSize: 18,
      fontWeight: "600",
      color: theme.text01,
    },
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

  const translateY = useSharedValue(screenHeight);
  const scrollY = useSharedValue(0);
  const overlayOpacity = useSharedValue(0);
  const scrollViewRef = useAnimatedRef<ScrollView>();

  const selectedIndex = useMemo(() => {
    if (!selectedValue) return 0;
    const index = items.findIndex((item) => item.value === selectedValue);
    return index >= 0 ? index : 0;
  }, [items, selectedValue]);

  const initialScrollY = selectedIndex * ITEM_HEIGHT;

  const showModal = useCallback(() => {
    overlayOpacity.value = withTiming(1, { duration: 300 });
    translateY.value = withTiming(0, { duration: 300 });
  }, [translateY, overlayOpacity]);

  const hideModal = useCallback(() => {
    overlayOpacity.value = withTiming(0, { duration: 300 });
    translateY.value = withTiming(screenHeight, { duration: 300 }, () => {
      runOnJS(onCancel)();
    });
  }, [translateY, overlayOpacity, onCancel]);

  useEffect(() => {
    if (visible) {
      showModal();
    } else {
      hideModal();
    }
  }, [visible, showModal, hideModal]);

  const animatedStyle = useAnimatedStyle(() => ({
    transform: [{ translateY: translateY.value }],
  }));

  const overlayAnimatedStyle = useAnimatedStyle(() => ({
    opacity: overlayOpacity.value,
  }));

  const scrollHandler = useAnimatedScrollHandler({
    onScroll: (event) => {
      scrollY.value = event.contentOffset.y;
    },
  });

  const handleDone = useCallback(() => {
    const currentIndex = Math.round(scrollY.value / ITEM_HEIGHT);
    const clampedIndex = Math.max(0, Math.min(currentIndex, items.length - 1));
    onSelect(items[clampedIndex]);
    hideModal();
  }, [scrollY, items, onSelect, hideModal]);

  const handleCancel = useCallback(() => {
    hideModal();
  }, [hideModal]);

  const renderItem = useCallback(
    (item: PickerItem) => {
      const isSelected = item.value === selectedValue;
      // const isSelected = Math.round(scrollY.value / ITEM_HEIGHT) === index;
      return (
        <View
          key={item.value}
          testID={`picker-item-${item.value || "empty"}`}
          style={styles.wheelItem}
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
      <Animated.View style={[styles.overlay, overlayAnimatedStyle]}>
        <Pressable style={styles.mask} onPress={handleCancel}></Pressable>
        <Animated.View style={[styles.modalContainer, animatedStyle]}>
          <View style={styles.header}>
            <TouchableOpacity testID="picker-cancel" onPress={handleCancel}>
              <Text style={styles.cancelButton}>{cancelButtonText}</Text>
            </TouchableOpacity>
            <Text style={styles.title}>{title}</Text>
            <TouchableOpacity testID="picker-confirm" onPress={handleDone}>
              <Text style={styles.doneButton}>{confirmButtonText}</Text>
            </TouchableOpacity>
          </View>

          <View style={styles.wheelContainer}>
            <View style={styles.selectionIndicator} />

            <AnimatedScrollView
              ref={scrollViewRef}
              style={styles.wheel}
              showsVerticalScrollIndicator={false}
              snapToInterval={ITEM_HEIGHT}
              decelerationRate="fast"
              onScroll={scrollHandler}
              scrollEventThrottle={16}
              contentOffset={{ x: 0, y: initialScrollY }}
            >
              {/* Add padding items to center the first and last items */}
              <View style={{ height: (WHEEL_HEIGHT - ITEM_HEIGHT) / 2 }} />
              {items.map(renderItem)}
              <View style={{ height: (WHEEL_HEIGHT - ITEM_HEIGHT) / 2 }} />
            </AnimatedScrollView>

            {/* Fade gradients for better UX */}
            <View style={[styles.fadeGradient, styles.fadeGradientTop]} />
            <View style={[styles.fadeGradient, styles.fadeGradientBottom]} />
          </View>
        </Animated.View>
      </Animated.View>
    </Modal>
  );
};

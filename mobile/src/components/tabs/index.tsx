import React, { useState, useRef, useCallback, useEffect } from "react";
import {
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  ScrollView,
  useWindowDimensions,
  ViewStyle,
  TextStyle,
  LayoutChangeEvent,
  NativeSyntheticEvent,
} from "react-native";

import PagerView from "react-native-pager-view";
import { horizontalSwipeOwnerTouchProps } from "@/common/horizontal-swipe-owner";
import { gutter, space, useTheme } from "@/common/theme";
import { ColorTheme } from "@/types/theme-props";

interface PageSelectedEvent {
  position: number;
}

export interface TabItem {
  key: string;
  title: string;
  component: React.ReactNode;
}

export interface TabsProps {
  tabs: TabItem[];
  initialIndex?: number;
  onTabChange?: (index: number) => void;
  tabBarStyle?: ViewStyle;
  tabStyle?: ViewStyle;
  activeTabStyle?: ViewStyle;
  inactiveTabStyle?: ViewStyle;
  tabTextStyle?: TextStyle;
  activeTabTextStyle?: TextStyle;
  inactiveTabTextStyle?: TextStyle;
  showIndicator?: boolean;
  indicatorStyle?: ViewStyle;
  scrollable?: boolean;
  autoScrollToCenter?: boolean;
  tabBarBackgroundColor?: string;
  contentContainerStyle?: ViewStyle;
}

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    container: {
      flex: 1,
      flexDirection: "column",
      justifyContent: "flex-start",
      backgroundColor: theme.white,
    },
    tabBar: {
      flexDirection: "row",
      backgroundColor: theme.white,
      borderBottomWidth: StyleSheet.hairlineWidth,
      borderBottomColor: theme.black90,
    },
    scrollableTabBar: {
      flexGrow: 0,
    },
    tab: {
      paddingVertical: space.lg,
      paddingHorizontal: gutter,
      alignItems: "center",
      justifyContent: "center",
      minWidth: 80,
    },
    activeTab: {
      borderBottomWidth: 2,
      borderBottomColor: theme.primary,
    },
    inactiveTab: {
      borderBottomWidth: 2,
      borderBottomColor: "transparent",
    },
    tabText: {
      fontSize: 16,
      fontWeight: "500",
      textAlign: "center",
    },
    activeTabText: {
      color: theme.primary,
      fontWeight: "600",
    },
    inactiveTabText: {
      color: theme.black80,
    },
    indicator: {
      position: "absolute",
      bottom: 0,
      height: 2,
      backgroundColor: theme.primary,
      borderRadius: 1,
    },
    pagerContainer: {
      flex: 1,
    },
    pagerView: {
      flex: 1,
    },
  });

export function Tabs({
  tabs,
  initialIndex = 0,
  onTabChange,
  tabBarStyle,
  tabStyle,
  activeTabStyle,
  inactiveTabStyle,
  tabTextStyle,
  activeTabTextStyle,
  inactiveTabTextStyle,
  scrollable = false,
  autoScrollToCenter = true,
  tabBarBackgroundColor,
  contentContainerStyle,
}: TabsProps) {
  const theme = useTheme().colorTheme;
  const { width: screenWidth } = useWindowDimensions();
  const styles = getStyles(theme);
  const [activeIndex, setActiveIndex] = useState(initialIndex);
  const pagerRef = useRef<PagerView>(null);
  const scrollViewRef = useRef<ScrollView>(null);
  const tabLayouts = useRef<{ [key: number]: { x: number; width: number } }>(
    {},
  );

  const handleTabPress = useCallback(
    (index: number) => {
      setActiveIndex(index);
      pagerRef.current?.setPage(index);
      onTabChange?.(index);
    },
    [onTabChange],
  );

  const handlePageSelected = useCallback(
    (event: NativeSyntheticEvent<PageSelectedEvent>) => {
      const index = event.nativeEvent.position;
      setActiveIndex(index);
      onTabChange?.(index);
    },
    [onTabChange],
  );

  const scrollToTab = useCallback(
    (index: number) => {
      if (!scrollable || !autoScrollToCenter) return;

      const layout = tabLayouts.current[index];
      if (layout) {
        const centerX = layout.x - (screenWidth - layout.width) / 2;
        scrollViewRef.current?.scrollTo({
          x: Math.max(0, centerX),
          animated: true,
        });
      }
    },
    [scrollable, autoScrollToCenter],
  );

  const handleTabLayout = useCallback(
    (index: number, event: LayoutChangeEvent) => {
      const { x, width } = event.nativeEvent.layout;
      tabLayouts.current[index] = { x, width };
    },
    [],
  );

  useEffect(() => {
    scrollToTab(activeIndex);
  }, [activeIndex, scrollToTab]);

  const renderTabBar = () => {
    const tabBarContent = tabs.map((tab, index) => {
      const isActive = index === activeIndex;
      return (
        <TouchableOpacity
          key={tab.key}
          style={[
            styles.tab,
            tabStyle,
            isActive
              ? [styles.activeTab, activeTabStyle]
              : [styles.inactiveTab, inactiveTabStyle],
          ]}
          onPress={() => handleTabPress(index)}
          onLayout={(event) => handleTabLayout(index, event)}
        >
          <Text
            style={[
              styles.tabText,
              tabTextStyle,
              isActive
                ? [styles.activeTabText, activeTabTextStyle]
                : [styles.inactiveTabText, inactiveTabTextStyle],
            ]}
          >
            {tab.title}
          </Text>
        </TouchableOpacity>
      );
    });

    if (scrollable) {
      return (
        <View style={styles.tabBar}>
          <ScrollView
            ref={scrollViewRef}
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={[styles.scrollableTabBar, tabBarStyle]}
            style={{ backgroundColor: tabBarBackgroundColor || theme.white }}
          >
            {tabBarContent}
          </ScrollView>
        </View>
      );
    }

    return (
      <View
        style={[
          styles.tabBar,
          tabBarStyle,
          { backgroundColor: tabBarBackgroundColor || theme.white },
        ]}
      >
        {tabBarContent}
      </View>
    );
  };

  return (
    // Swipes over the tab bar or pager belong to this component.
    <View
      style={[styles.container, contentContainerStyle]}
      {...horizontalSwipeOwnerTouchProps}
    >
      {renderTabBar()}
      <View style={styles.pagerContainer}>
        <PagerView
          ref={pagerRef}
          style={styles.pagerView}
          initialPage={initialIndex}
          onPageSelected={handlePageSelected}
          scrollEnabled={true}
        >
          {tabs.map((tab) => (
            <View key={tab.key} style={{ flex: 1 }}>
              {tab.component}
            </View>
          ))}
        </PagerView>
      </View>
    </View>
  );
}

export default Tabs;

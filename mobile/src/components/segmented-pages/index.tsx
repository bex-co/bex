import { ReactNode, useCallback, useRef, useState } from "react";
import {
  NativeSyntheticEvent,
  ScrollView,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from "react-native";
import PagerView from "react-native-pager-view";
import { horizontalSwipeOwnerTouchProps } from "@/common/horizontal-swipe-owner";
import { fontSizes, fontWeights } from "@/common/theme";
import { ColorTheme } from "@/types/theme-props";
import { useThemeStyle } from "@/common/hooks/use-theme-style";

interface PageSelectedEvent {
  position: number;
}

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    // The tab strip scrolls horizontally: at 16px three labels can outrun the
    // screen in longer locales (de: "Net Worth" + "Vermögen" +
    // "Verbindlichkeiten"), and truncating a tab name reads worse than a nudge.
    tabsRow: {
      flexGrow: 0,
      marginBottom: 12,
    },
    tabsContent: {
      paddingHorizontal: 16,
    },
    tab: {
      paddingHorizontal: 12,
      paddingVertical: 6,
      borderRadius: 16,
      marginRight: 4,
    },
    tabActive: {
      backgroundColor: theme.black20,
    },
    label: {
      fontSize: fontSizes.lg,
      fontWeight: fontWeights.regular,
      color: theme.black80,
    },
    labelActive: {
      fontWeight: fontWeights.medium,
      color: theme.text01,
    },
    pager: {
      width: "100%",
    },
  });

type SegmentedPagesProps = {
  /** Tab labels, one per page (same length and order as `pages`). */
  tabs: string[];
  /** The pages to render, one per tab. */
  pages: ReactNode[];
  /** Fixed height for the pager (PagerView needs a bounded height). */
  height: number;
  initialIndex?: number;
  onPageChange?: (index: number) => void;
};

/**
 * Pages switched by a segmented tab strip at the top — tap only, no swiping.
 * Horizontal gestures are left entirely to the page content (the net-worth
 * chart's scrub), which is why the dashboard card uses this over a swipeable
 * carousel.
 */
export function SegmentedPages({
  tabs,
  pages,
  height,
  initialIndex = 0,
  onPageChange,
}: SegmentedPagesProps): React.ReactElement {
  const styles = useThemeStyle(getStyles);
  const pagerRef = useRef<PagerView>(null);
  const [activeIndex, setActiveIndex] = useState(initialIndex);

  const handlePageSelected = useCallback(
    (event: NativeSyntheticEvent<PageSelectedEvent>) => {
      const index = event.nativeEvent.position;
      setActiveIndex(index);
      onPageChange?.(index);
    },
    [onPageChange],
  );

  // Highlight moves on touch (setPage's onPageSelected lands a frame later),
  // but onPageChange fires only from onPageSelected so a switch reports once.
  const handleTabPress = useCallback(
    (index: number) => {
      if (index === activeIndex) return;
      setActiveIndex(index);
      pagerRef.current?.setPage(index);
    },
    [activeIndex],
  );

  return (
    <View>
      {/* A horizontal drag across the tab strip belongs to this component. */}
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        style={styles.tabsRow}
        contentContainerStyle={styles.tabsContent}
        accessibilityRole="tablist"
        {...horizontalSwipeOwnerTouchProps}
      >
        {tabs.map((tab, index) => {
          const active = index === activeIndex;
          return (
            <TouchableOpacity
              key={tab}
              style={[styles.tab, active && styles.tabActive]}
              onPress={() => handleTabPress(index)}
              accessibilityRole="tab"
              accessibilityState={{ selected: active }}
            >
              <Text
                style={[styles.label, active && styles.labelActive]}
                numberOfLines={1}
              >
                {tab}
              </Text>
            </TouchableOpacity>
          );
        })}
      </ScrollView>
      <PagerView
        ref={pagerRef}
        style={[styles.pager, { height }]}
        initialPage={initialIndex}
        onPageSelected={handlePageSelected}
        scrollEnabled={false}
      >
        {pages.map((page, index) => (
          <View key={index} style={{ flex: 1 }}>
            {page}
          </View>
        ))}
      </PagerView>
    </View>
  );
}

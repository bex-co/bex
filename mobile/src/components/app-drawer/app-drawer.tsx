import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  AccessibilityInfo,
  Animated,
  BackHandler,
  findNodeHandle,
  Image,
  PanResponder,
  Pressable,
  StyleSheet,
  Text,
  useWindowDimensions,
  View,
} from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { fontSizes, gutter, space } from "@/common/theme";
import { useThemeStyle } from "@/common/hooks/use-theme-style";
import { useReducedMotion } from "@/common/hooks/use-reduced-motion";
import { useTranslations } from "@/common/hooks/use-translations";
import { hasActiveHorizontalSwipeOwnerTouch } from "@/common/horizontal-swipe-owner";
import type { ColorTheme } from "@/types/theme-props";
import {
  animationDuration,
  clamp,
  CLOSE_DURATION_MS,
  drawerWidthFor,
  OPEN_DURATION_MS,
  resolveRelease,
  shouldCaptureGesture,
  SNAP_BACK_DURATION_MS,
} from "./app-drawer-gestures";
import { PersonalFooter } from "./personal-footer";
import { WorkspaceList } from "./workspace-list";

const getStyles = (theme: ColorTheme) =>
  StyleSheet.create({
    root: { flex: 1, backgroundColor: theme.background },
    // Stationary menu layer beneath the app content; the content slides right
    // to reveal it (no Modal — nothing overlays the app when closed).
    drawerLayer: {
      position: "absolute",
      top: 0,
      left: 0,
      bottom: 0,
      backgroundColor: theme.card,
    },
    content: { flex: 1, backgroundColor: theme.background },
    contentOpen: {
      shadowColor: "#000",
      shadowOffset: { width: -4, height: 0 },
      shadowOpacity: 0.15,
      shadowRadius: 12,
      elevation: 16,
    },
    brandRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: space.sm,
      paddingHorizontal: gutter,
      minHeight: 60,
      paddingVertical: space.sm,
    },
    brandMark: {
      width: 32,
      height: 32,
      borderRadius: space.sm,
    },
    brandText: {
      fontSize: fontSizes.xl,
      fontWeight: "700",
      color: theme.foreground,
    },
  });

type AppDrawerProps = {
  open: boolean;
  onOpen: () => void;
  onClose: () => void;
  children: ReactNode;
};

/**
 * Reveal drawer for the authenticated app: the workspace/account menu is a
 * stationary layer at the bottom of the tree and the app content slides right
 * to uncover it. Opens by a rightward swipe from the content's left edge (or
 * the header button), closes by tapping the visible content sliver, a leftward
 * swipe, or Android back. Adapted from the Beancount ledger drawer, re-themed,
 * and hardened for reduced motion, safe areas, rotation, and screen readers.
 */
export function AppDrawer({ open, onOpen, onClose, children }: AppDrawerProps) {
  const styles = useThemeStyle(getStyles);
  const { t } = useTranslations();
  const { width: windowWidth } = useWindowDimensions();
  const drawerWidth = drawerWidthFor(windowWidth);
  const insets = useSafeAreaInsets();
  const reducedMotion = useReducedMotion();

  // Gates the menu layer and the tap-to-close catcher; stays true through the
  // close animation so the reveal doesn't pop.
  const [visible, setVisible] = useState(open);
  const progress = useRef(new Animated.Value(0)).current;
  // Each show/hide bumps the generation so a superseded hide's completion
  // callback can't unmount the wrong state.
  const generationRef = useRef(0);
  const openRef = useRef(open);
  openRef.current = open;
  const reducedRef = useRef(reducedMotion);
  reducedRef.current = reducedMotion;
  const drawerLayerRef = useRef<View>(null);

  const hide = useCallback(() => {
    const generation = ++generationRef.current;
    Animated.timing(progress, {
      toValue: 0,
      duration: animationDuration(CLOSE_DURATION_MS, reducedRef.current),
      useNativeDriver: true,
    }).start(() => {
      // Deliberately ignore `finished`: even an interrupted close must end
      // hidden so the tap catcher stops eating touches.
      if (generationRef.current === generation && !openRef.current) {
        setVisible(false);
      }
    });
  }, [progress]);

  const show = useCallback(() => {
    generationRef.current += 1;
    setVisible(true);
    Animated.timing(progress, {
      toValue: 1,
      duration: animationDuration(OPEN_DURATION_MS, reducedRef.current),
      useNativeDriver: true,
    }).start();
  }, [progress]);

  useEffect(() => {
    if (open) show();
    else hide();
  }, [open, show, hide]);

  // Move screen-reader focus into the menu on open and announce it; the system
  // restores focus toward the trigger on close (best effort).
  useEffect(() => {
    if (!open) return;
    AccessibilityInfo.announceForAccessibility(t("drawer.opened"));
    const node = findNodeHandle(drawerLayerRef.current);
    if (node != null) AccessibilityInfo.setAccessibilityFocus(node);
  }, [open, t]);

  // Escape hatch: if the parent already flipped to closed but the catcher is
  // still up (a missed close), a repeat dismiss force-hides instead of
  // no-opping on unchanged parent state.
  const requestClose = useCallback(() => {
    if (openRef.current) onClose();
    else hide();
  }, [onClose, hide]);

  useEffect(() => {
    if (!open) return;
    const subscription = BackHandler.addEventListener(
      "hardwareBackPress",
      () => {
        requestClose();
        return true;
      },
    );
    return () => subscription.remove();
  }, [open, requestClose]);

  // Bidirectional content drag. Closed: rightward drags from the left-edge
  // strip pull the drawer open. Open: leftward drags anywhere push it shut.
  // Taps, vertical scrolls, and horizontal-swipe owners never become a drag.
  const dragStartRef = useRef(1);
  const panResponder = useMemo(
    () =>
      PanResponder.create({
        onMoveShouldSetPanResponder: (_evt, gesture) =>
          shouldCaptureGesture(
            openRef.current,
            gesture,
            hasActiveHorizontalSwipeOwnerTouch(),
          ),
        onPanResponderTerminationRequest: () => false,
        onPanResponderGrant: () => {
          if (!openRef.current) {
            generationRef.current += 1;
            setVisible(true);
          }
          progress.stopAnimation((value) => {
            dragStartRef.current = value;
          });
        },
        onPanResponderMove: (_evt, gesture) => {
          progress.setValue(
            clamp(dragStartRef.current + gesture.dx / drawerWidth, 0, 1),
          );
        },
        onPanResponderRelease: (_evt, gesture) => {
          const outcome = resolveRelease(openRef.current, gesture, drawerWidth);
          if (outcome === "close") return requestClose();
          if (outcome === "open") return onOpen();
          Animated.timing(progress, {
            toValue: outcome === "snap-open" ? 1 : 0,
            duration: animationDuration(
              SNAP_BACK_DURATION_MS,
              reducedRef.current,
            ),
            useNativeDriver: true,
          }).start(() => {
            if (outcome === "snap-closed" && !openRef.current) {
              setVisible(false);
            }
          });
        },
        onPanResponderTerminate: () => {
          if (openRef.current) {
            Animated.timing(progress, {
              toValue: 1,
              duration: animationDuration(
                SNAP_BACK_DURATION_MS,
                reducedRef.current,
              ),
              useNativeDriver: true,
            }).start();
          } else {
            hide();
          }
        },
      }),
    [progress, drawerWidth, requestClose, onOpen, hide],
  );

  // Memoized so re-renders mid-animation don't swap the animated node out from
  // under the native driver.
  const translateX = useMemo(
    () =>
      progress.interpolate({
        inputRange: [0, 1],
        outputRange: [0, drawerWidth],
      }),
    [progress, drawerWidth],
  );

  return (
    <View style={styles.root}>
      {visible ? (
        <View
          ref={drawerLayerRef}
          accessible={false}
          accessibilityViewIsModal={open}
          accessibilityLabel={t("drawer.menuLabel")}
          style={[
            styles.drawerLayer,
            {
              width: drawerWidth,
              paddingTop: Math.max(insets.top, space.md),
              paddingBottom: insets.bottom,
            },
          ]}
        >
          <View style={styles.brandRow}>
            <Image
              source={require("@/assets/images/logo.png")}
              style={styles.brandMark}
              resizeMode="contain"
              accessible={false}
            />
            <Text style={styles.brandText}>bex</Text>
          </View>
          <WorkspaceList onSelected={requestClose} />
          <PersonalFooter />
        </View>
      ) : null}

      <Animated.View
        accessibilityElementsHidden={open}
        importantForAccessibility={open ? "no-hide-descendants" : "auto"}
        style={[
          styles.content,
          visible && styles.contentOpen,
          { transform: [{ translateX }] },
        ]}
        {...panResponder.panHandlers}
      >
        {children}
        {visible ? (
          <Pressable
            accessibilityRole="button"
            accessibilityLabel={t("drawer.closeBackdrop")}
            style={StyleSheet.absoluteFill}
            onPress={requestClose}
          />
        ) : null}
      </Animated.View>
    </View>
  );
}

import type { GestureResponderEvent } from "react-native";

/**
 * Tracks in-flight touches that began inside a component that owns horizontal
 * swipes (interactive charts, paged carousels, swipeable tabs). A parent
 * navigator can check this before claiming an edge swipe, so a local gesture
 * always wins over the global navigation gesture — even where the local
 * component doesn't claim the responder itself (chart headers, pills, dots).
 *
 * A plain module-level counter, not React state: it is read synchronously
 * inside gesture negotiation callbacks and must never trigger re-renders.
 */
let activeTouchCount = 0;

export function hasActiveHorizontalSwipeOwnerTouch(): boolean {
  return activeTouchCount > 0;
}

function releaseTouch(event: GestureResponderEvent): void {
  // When the last finger lifts, reset outright so a missed event can never
  // leave the parent edge-swipe permanently disabled.
  if (event.nativeEvent.touches.length === 0) {
    activeTouchCount = 0;
  } else {
    activeTouchCount = Math.max(0, activeTouchCount - 1);
  }
}

/**
 * Spread onto the root <View> of a component whose content handles horizontal
 * swipes itself. These are passive touch listeners — they don't join responder
 * negotiation. Touch events bubble through the original target's ancestors, so
 * nested owners (a chart inside a carousel) increment and release in balance.
 */
export const horizontalSwipeOwnerTouchProps = {
  onTouchStart: (): void => {
    activeTouchCount += 1;
  },
  onTouchEnd: releaseTouch,
  onTouchCancel: releaseTouch,
} as const;

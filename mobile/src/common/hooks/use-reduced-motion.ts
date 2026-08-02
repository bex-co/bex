import { useEffect, useState } from "react";
import { AccessibilityInfo } from "react-native";

/**
 * Tracks the OS "reduce motion" accessibility preference. The custom reveal
 * drawer uses this to collapse its open/close animation to an instant state
 * change, so users who ask for reduced motion never see the slide.
 *
 * Starts pessimistic-free (motion allowed) and repairs from the platform on
 * mount; a missing/older native module simply leaves motion enabled rather
 * than throwing.
 */
export function useReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    let active = true;
    AccessibilityInfo.isReduceMotionEnabled?.()
      .then((value) => {
        if (active) setReduced(value);
      })
      .catch(() => undefined);
    const subscription = AccessibilityInfo.addEventListener(
      "reduceMotionChanged",
      (value) => setReduced(value),
    );
    return () => {
      active = false;
      subscription.remove();
    };
  }, []);
  return reduced;
}

import { useEffect, useRef } from "react";
import { cn } from "@/common/lib/utils/utils";

/**
 * A project page section Render's `/project/<id>/environment/<envId>` names
 * (w1/m682): while `focused` holds, it scrolls to the section's top, again
 * each time `settle` changes (rows above it arrive and push it down). Returns
 * the section's ref.
 */
export function useFocusedSection(focused: boolean, settle: unknown) {
  const ref = useRef<HTMLElement>(null);
  useEffect(() => {
    if (!focused) return;
    const reduce = window.matchMedia?.(
      "(prefers-reduced-motion: reduce)",
    ).matches;
    ref.current?.scrollIntoView?.({
      behavior: reduce ? "auto" : "smooth",
      block: "start",
    });
  }, [focused, settle]);
  return ref;
}

/** Render's outline 8px off a focused section's edge, which fades as focus clears. */
export function focusedSectionClass(focused: boolean): string {
  return cn(
    "scroll-mt-6 rounded-lg outline outline-offset-8 motion-safe:transition-[outline-color] motion-safe:duration-500",
    focused ? "outline-primary" : "outline-transparent",
  );
}

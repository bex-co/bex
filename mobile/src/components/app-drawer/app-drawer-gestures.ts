/**
 * Pure gesture/animation math for the reveal drawer. Kept free of React and
 * react-native so the interaction rules — edge-only open, horizontal
 * dominance, owner deference, snap thresholds, reduced-motion timing — can be
 * unit-tested without rendering the drawer.
 */

/** Slide-open animation length (ms) at full motion. */
export const OPEN_DURATION_MS = 240;
/** Slide-closed animation length (ms) at full motion. */
export const CLOSE_DURATION_MS = 200;
/** Snap-back length (ms) when a drag is released without crossing threshold. */
export const SNAP_BACK_DURATION_MS = 150;
/**
 * Rightward swipes only open the drawer when they begin in this strip along the
 * content's left edge, so horizontal carousels, chips and pagers keep working.
 */
export const EDGE_HIT_WIDTH = 32;
/** Upper bound for the menu width on wide/rotated screens. */
export const MAX_DRAWER_WIDTH = 340;
/** Fraction of the window the menu occupies below the upper bound. */
export const DRAWER_WIDTH_FRACTION = 0.85;

export const clamp = (value: number, min: number, max: number): number =>
  Math.min(max, Math.max(min, value));

/**
 * Responsive menu width: a fraction of the window, capped so it never spans a
 * tablet/landscape screen. Recomputed on rotation by passing the live window
 * width.
 */
export function drawerWidthFor(windowWidth: number): number {
  return Math.min(MAX_DRAWER_WIDTH, windowWidth * DRAWER_WIDTH_FRACTION);
}

/** A pan gesture, reduced to the fields the drawer negotiates on. */
export type DrawerGesture = {
  /** Total horizontal travel from the touch origin. */
  dx: number;
  /** Total vertical travel from the touch origin. */
  dy: number;
  /** Touch origin x, in content coordinates. */
  x0: number;
  /** Release horizontal velocity. */
  vx: number;
};

/**
 * Decide whether the drawer should claim a move gesture as the pan responder.
 * Vertical scrolls and taps never qualify; when open, only a decisive leftward
 * drag closes; when closed, only a rightward drag that starts in the edge strip
 * and does not belong to a horizontal-swipe owner opens.
 */
export function shouldCaptureGesture(
  open: boolean,
  gesture: DrawerGesture,
  hasOwnerTouch: boolean,
): boolean {
  const isHorizontal = Math.abs(gesture.dx) > Math.abs(gesture.dy) * 1.5;
  if (!isHorizontal) return false;
  if (open) return gesture.dx < -8;
  if (hasOwnerTouch) return false;
  return gesture.x0 <= EDGE_HIT_WIDTH && gesture.dx > 8;
}

export type ReleaseOutcome = "open" | "close" | "snap-open" | "snap-closed";

/**
 * Resolve a released drag into a terminal drawer action. A flick (velocity) or
 * crossing a third of the drawer width commits; otherwise it snaps back to the
 * state it started in.
 */
export function resolveRelease(
  open: boolean,
  gesture: DrawerGesture,
  drawerWidth: number,
): ReleaseOutcome {
  if (open) {
    const shouldClose = gesture.vx < -0.3 || gesture.dx < -drawerWidth / 3;
    return shouldClose ? "close" : "snap-open";
  }
  const shouldOpen = gesture.vx > 0.3 || gesture.dx > drawerWidth / 3;
  return shouldOpen ? "open" : "snap-closed";
}

/**
 * Animation duration honoring the reduce-motion preference: reduced motion
 * collapses every transition to an instant (0ms) state change.
 */
export function animationDuration(
  base: number,
  reducedMotion: boolean,
): number {
  return reducedMotion ? 0 : base;
}

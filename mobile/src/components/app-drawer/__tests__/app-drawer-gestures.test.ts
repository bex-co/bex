import {
  animationDuration,
  clamp,
  CLOSE_DURATION_MS,
  drawerWidthFor,
  EDGE_HIT_WIDTH,
  MAX_DRAWER_WIDTH,
  resolveRelease,
  shouldCaptureGesture,
} from "../app-drawer-gestures";

const gesture = (
  over: Partial<Parameters<typeof shouldCaptureGesture>[1]>,
) => ({
  dx: 0,
  dy: 0,
  x0: 0,
  vx: 0,
  ...over,
});

describe("drawer gesture capture", () => {
  it("opens only from a horizontal drag that begins in the left edge strip", () => {
    expect(
      shouldCaptureGesture(false, gesture({ x0: 4, dx: 20, dy: 2 }), false),
    ).toBe(true);
    // Started past the edge strip → not a drawer open (a carousel/chip swipe).
    expect(
      shouldCaptureGesture(
        false,
        gesture({ x0: EDGE_HIT_WIDTH + 5, dx: 20, dy: 2 }),
        false,
      ),
    ).toBe(false);
  });

  it("defers to an active horizontal-swipe owner even at the edge", () => {
    expect(
      shouldCaptureGesture(false, gesture({ x0: 2, dx: 30, dy: 1 }), true),
    ).toBe(false);
  });

  it("ignores vertical scrolls and taps in both states", () => {
    expect(
      shouldCaptureGesture(false, gesture({ x0: 2, dx: 10, dy: 40 }), false),
    ).toBe(false);
    expect(shouldCaptureGesture(true, gesture({ dx: -4, dy: 40 }), false)).toBe(
      false,
    );
  });

  it("closes only on a decisive leftward drag when open", () => {
    expect(shouldCaptureGesture(true, gesture({ dx: -20, dy: 2 }), false)).toBe(
      true,
    );
    expect(shouldCaptureGesture(true, gesture({ dx: 20, dy: 2 }), false)).toBe(
      false,
    );
  });
});

describe("drawer release resolution", () => {
  const width = 300;
  it("commits open past the third-width threshold or a flick", () => {
    expect(resolveRelease(false, gesture({ dx: width / 2 }), width)).toBe(
      "open",
    );
    expect(resolveRelease(false, gesture({ dx: 10, vx: 0.5 }), width)).toBe(
      "open",
    );
  });
  it("snaps closed on a weak open drag", () => {
    expect(resolveRelease(false, gesture({ dx: 20, vx: 0.1 }), width)).toBe(
      "snap-closed",
    );
  });
  it("commits close past the threshold or a leftward flick", () => {
    expect(resolveRelease(true, gesture({ dx: -width / 2 }), width)).toBe(
      "close",
    );
    expect(resolveRelease(true, gesture({ dx: -10, vx: -0.5 }), width)).toBe(
      "close",
    );
  });
  it("snaps back open on a weak close drag", () => {
    expect(resolveRelease(true, gesture({ dx: -20, vx: -0.1 }), width)).toBe(
      "snap-open",
    );
  });
});

describe("drawer width and motion", () => {
  it("caps the width on wide/rotated screens and scales on narrow ones", () => {
    expect(drawerWidthFor(1200)).toBe(MAX_DRAWER_WIDTH);
    expect(drawerWidthFor(300)).toBeCloseTo(255);
  });
  it("collapses animation to instant under reduced motion", () => {
    expect(animationDuration(CLOSE_DURATION_MS, true)).toBe(0);
    expect(animationDuration(CLOSE_DURATION_MS, false)).toBe(CLOSE_DURATION_MS);
  });
  it("clamps drag progress into the unit interval", () => {
    expect(clamp(1.4, 0, 1)).toBe(1);
    expect(clamp(-0.2, 0, 1)).toBe(0);
    expect(clamp(0.5, 0, 1)).toBe(0.5);
  });
});

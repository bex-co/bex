import { ClientOnly } from "@tanstack/react-router";
import { useEffect } from "react";

const VisualViewportHeightImpl = () => {
  useEffect(() => {
    const vv = window.visualViewport;
    if (!vv) return;

    let rafId = 0;

    const update = () => {
      cancelAnimationFrame(rafId);
      rafId = requestAnimationFrame(() => {
        // Multiply by scale to compensate for pinch-zoom: we want the
        // layout-level height minus the keyboard, not the zoomed-in
        // visual height.
        const height = vv.height * vv.scale;
        document.documentElement.style.setProperty(
          "--visual-viewport-height",
          `${height}px`,
        );
      });
    };

    // Set initial value
    update();

    // Listen to both resize and scroll: on iOS Safari, when the keyboard
    // opens and the browser scrolls to keep the focused input visible,
    // only a scroll event fires (not resize).
    vv.addEventListener("resize", update);
    vv.addEventListener("scroll", update);

    return () => {
      vv.removeEventListener("resize", update);
      vv.removeEventListener("scroll", update);
      cancelAnimationFrame(rafId);
      document.documentElement.style.removeProperty("--visual-viewport-height");
    };
  }, []);
  return null;
};

export const VisualViewportHeight = () => {
  return (
    <ClientOnly>
      <VisualViewportHeightImpl />
    </ClientOnly>
  );
};

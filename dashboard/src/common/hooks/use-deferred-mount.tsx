import { useEffect, useRef, useState, type RefObject } from "react";

export interface UseDeferredMountOptions {
  /** Expand the intersection root so near-viewport sections warm early. */
  rootMargin?: string;
  /**
   * Mount immediately when true (e.g. parent already knows the section is
   * on-screen). Hash targeting uses a post-mount check to avoid SSR/client
   * hydration disagreement.
   */
  eager?: boolean;
  /** Section id without `#`; if the location hash matches, mount after paint. */
  hashId?: string;
}

/**
 * Delays mounting heavy below-the-fold UI until the sentinel enters (or is near)
 * the viewport. SSR and the first client paint stay empty so Apollo polls and
 * large panel graphs do not start until the user scrolls near them.
 */
export function useDeferredMount({
  rootMargin = "240px",
  eager = false,
  hashId,
}: UseDeferredMountOptions = {}): {
  ref: RefObject<HTMLDivElement | null>;
  mounted: boolean;
} {
  const ref = useRef<HTMLDivElement | null>(null);
  const [mounted, setMounted] = useState(eager);

  useEffect(() => {
    if (mounted) return;

    if (hashId && window.location.hash === `#${hashId}`) {
      let cancelled = false;
      queueMicrotask(() => {
        if (!cancelled) setMounted(true);
      });
      return () => {
        cancelled = true;
      };
    }

    const node = ref.current;
    if (!node) return;

    if (typeof IntersectionObserver === "undefined") {
      let cancelled = false;
      queueMicrotask(() => {
        if (!cancelled) setMounted(true);
      });
      return () => {
        cancelled = true;
      };
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) {
          setMounted(true);
          observer.disconnect();
        }
      },
      { rootMargin },
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [mounted, rootMargin, hashId]);

  return { ref, mounted };
}

import type { ReactNode } from "react";

import {
  useDeferredMount,
  type UseDeferredMountOptions,
} from "@/common/hooks/use-deferred-mount";

/** Wraps children so they only mount once near the viewport (or via hash). */
export function DeferredMount({
  children,
  rootMargin,
  eager,
  hashId,
  className,
  minHeight,
}: UseDeferredMountOptions & {
  children: ReactNode;
  className?: string;
  /** Reserves layout space before mount so scroll position stays stable. */
  minHeight?: number | string;
}) {
  const { ref, mounted } = useDeferredMount({ rootMargin, eager, hashId });
  return (
    <div
      ref={ref}
      className={className}
      style={!mounted && minHeight != null ? { minHeight } : undefined}
    >
      {mounted ? children : null}
    </div>
  );
}

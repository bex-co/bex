import type { ReactNode } from "react";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";

/**
 * Disables every form control inside it until React has hydrated (w4/m219).
 * Server-rendered inputs are visible before their handlers exist: text typed
 * then is overwritten when hydration re-applies the controlled value — a draft
 * name vanishes, a Port edit snaps back to its default. A disabled fieldset is
 * native, so it holds in the server HTML itself; `display: contents` keeps it
 * out of layout, and after hydration (or on client navigation, which renders
 * hydrated from the start) it is an ordinary enabled fieldset.
 */
export function HydrationFieldset({ children }: { children: ReactNode }) {
  const hydrated = useIsHydrated();
  return (
    <fieldset
      disabled={!hydrated}
      aria-busy={!hydrated || undefined}
      className="contents"
    >
      {children}
    </fieldset>
  );
}

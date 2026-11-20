import { useIsHydrated } from "@/common/hooks/use-is-hydrated";

/**
 * Whether the reader is on a Mac (or an iPhone or iPad), for a shortcut's
 * label: `⌘` there, `Ctrl` elsewhere. False through the server render and the
 * hydration pass, which cannot know the platform, so the two agree (no React
 * #418); the real answer from the first render after.
 */
export function useIsMac(): boolean {
  const hydrated = useIsHydrated();
  return hydrated && /Mac|iPhone|iPad/.test(navigator.platform);
}

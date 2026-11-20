import { createContext, useContext, type ComponentProps } from "react";
import { cn } from "@/common/lib/utils/utils.ts";

/**
 * The theme classes of the nearest subtree that keeps a theme of its own (a
 * log explorer under Log Explorer Theme, w3/m531), like Render's `Theme`
 * context. A menu, select, popover, sheet or tooltip opened inside one
 * portals out of it, so its content reads this and carries the same classes:
 * Render's floating panels follow the explorer's theme the same way.
 */
const ThemeScopeContext = createContext<string | undefined>(undefined);

/** The enclosing scope's theme classes, or undefined outside one. */
// eslint-disable-next-line react-refresh/only-export-components -- the scope's reader belongs beside it
export const useThemeScopeClass = () => useContext(ThemeScopeContext);

/**
 * A `<div>` that keeps `theme` (`"dark"`, `"light high-contrast"`, …)
 * whatever the page's theme, or follows the page when `theme` is undefined.
 * The div and the provider are always rendered, so a change of theme never
 * remounts what is inside.
 */
export function ThemeScope({
  theme,
  className,
  ...props
}: ComponentProps<"div"> & { theme: string | undefined }) {
  const enclosing = useThemeScopeClass();
  return (
    <ThemeScopeContext.Provider value={theme ?? enclosing}>
      <div
        {...props}
        // The text colour is inherited from <body> already resolved, so a
        // scope sets its own from its tokens.
        className={cn(
          theme && [theme, "bg-background text-foreground"],
          className,
        )}
      />
    </ThemeScopeContext.Provider>
  );
}

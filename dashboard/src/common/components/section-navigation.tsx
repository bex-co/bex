import type { LucideIcon } from "lucide-react";
import { cn } from "@/common/lib/utils/utils";

export interface SectionNavigationItem {
  href: string;
  label: string;
  icon: LucideIcon;
}

export const SECTION_NAVIGATION_STICKY_CLASS =
  "sticky top-0 z-20 -mx-4 border-y bg-background/95 px-4 py-2 backdrop-blur sm:-mx-6 sm:px-6 lg:top-6 lg:col-start-2 lg:row-start-1 lg:mx-0 lg:border-0 lg:bg-transparent lg:px-0 lg:py-0 lg:backdrop-blur-none";

/**
 * Scroll margin for anything a link scrolls to beneath the section row. Below
 * lg the 54px row sticks at the scroll container's padding edge (p-4, sm:p-6,
 * or none on service settings), so it covers up to the top 78px of the
 * scrollport; 96px leaves the target's heading clear of it (w4/336). The lg
 * rail sits beside the content, which keeps the plain 24px. Usage and account
 * Settings copy the row's classes inline, so a height change there moves this.
 */
export const SECTION_NAVIGATION_TARGET_CLASS = "scroll-mt-24 lg:scroll-mt-6";

export const SECTION_NAVIGATION_ITEMS_CLASS =
  "flex gap-1 overflow-x-auto lg:flex-col lg:overflow-visible";

/**
 * Responsive in-page section navigation shared by long settings surfaces.
 * Placement (right rail vs. mobile sticky row) stays with the owning page;
 * this component owns the navigation semantics and link presentation.
 */
export function SectionNavigation({
  ariaLabel,
  items,
  className,
}: {
  ariaLabel: string;
  items: SectionNavigationItem[];
  className?: string;
}) {
  return (
    <nav aria-label={ariaLabel} className={cn("min-w-0", className)}>
      <div className={SECTION_NAVIGATION_ITEMS_CLASS}>
        {items.map(({ href, label, icon: Icon }) => (
          <a
            key={href}
            href={href}
            className="flex shrink-0 items-center gap-2 rounded-md px-3 py-2 text-sm font-medium whitespace-nowrap text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50 lg:whitespace-normal"
          >
            <Icon aria-hidden="true" className="size-4 shrink-0" />
            {label}
          </a>
        ))}
      </div>
    </nav>
  );
}

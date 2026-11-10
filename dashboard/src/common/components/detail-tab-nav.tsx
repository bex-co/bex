import { cn } from "@/common/lib/utils/utils";

export interface DetailTab {
  id: string;
  label: string;
  /** Exactly one tab of a nav is active. */
  active: boolean;
  onSelect: () => void;
}

/**
 * The tab row of a resource detail page (Postgres, Key Value): a labelled
 * `nav` of buttons that expose the current one with `aria-current="page"`, so
 * assistive technology hears which view is showing, not only a border class
 * (WCAG 4.1.2; w21/027).
 */
export function DetailTabNav({
  label,
  tabs,
  skeletonRegion,
}: {
  label: string;
  tabs: DetailTab[];
  /** Marks the nav as the "tabs" region of the page's loading skeleton. */
  skeletonRegion?: "tabs";
}) {
  return (
    <nav
      data-skeleton-region={skeletonRegion}
      aria-label={label}
      className="flex gap-1 border-b px-4 sm:px-6"
    >
      {tabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          aria-current={tab.active ? "page" : undefined}
          className={cn(
            "border-b-2 px-3 py-2 text-sm",
            tab.active
              ? "border-foreground text-foreground"
              : "border-transparent text-muted-foreground",
          )}
          onClick={tab.onSelect}
        >
          {tab.label}
        </button>
      ))}
    </nav>
  );
}

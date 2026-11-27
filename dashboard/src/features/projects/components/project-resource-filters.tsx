import { Search } from "lucide-react";
import { Input } from "@/common/components/ui/input";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { ReservedButton } from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { TOUCH_TARGET_HEIGHT } from "@/common/lib/touch-target";
import { cn } from "@/common/lib/utils/utils";
import type { EnvironmentResourceFilter } from "@/features/projects/lib/resource-filter";

const FILTER_KEYS: Array<{
  filter: EnvironmentResourceFilter;
  key:
    | "projects.filterAll"
    | "projects.filterServices"
    | "projects.filterEnvGroups";
}> = [
  { filter: "all", key: "projects.filterAll" },
  { filter: "services", key: "projects.filterServices" },
  { filter: "envgroups", key: "projects.filterEnvGroups" },
];

/**
 * An environment section's counted All / Services / Env Groups switcher, as
 * Render's project page draws it (w1/m682). Each count is the section's own,
 * before search.
 */
export function EnvironmentResourceSwitcher({
  filter,
  counts,
  onChange,
}: {
  filter: EnvironmentResourceFilter;
  counts: Record<EnvironmentResourceFilter, number | null>;
  onChange: (filter: EnvironmentResourceFilter) => void;
}) {
  const { t } = useTranslations();
  return (
    <div
      role="group"
      className="flex flex-wrap gap-1"
      aria-label={t("projects.filterTypeLabel")}
    >
      {FILTER_KEYS.map(({ filter: option, key }) => {
        // The variant is the only visual cue; assistive technology needs
        // the same state (w4/333).
        const selected = filter === option;
        return (
          <Button
            key={option}
            size="sm"
            variant={selected ? "default" : "outline"}
            aria-pressed={selected}
            onClick={() => onChange(option)}
          >
            {/* An unread family has no count, not zero (w4/m227). */}
            {t(key)} ({counts[option] ?? "—"})
          </Button>
        );
      })}
    </div>
  );
}

/** The section's "Search resources in <env>" box, which filters only it. */
export function EnvironmentResourceSearch({
  environmentName,
  query,
  onChange,
}: {
  environmentName: string;
  query: string;
  onChange: (query: string) => void;
}) {
  const { t } = useTranslations();
  return (
    <div className="relative">
      <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        type="search"
        value={query}
        onChange={(event) => onChange(event.target.value)}
        aria-label={t("projects.searchLabel", {
          environment: environmentName,
        })}
        placeholder={t("projects.searchPlaceholder", {
          environment: environmentName,
        })}
        className="pl-8"
      />
    </div>
  );
}

/**
 * The switcher's loading preview (w4/715): the three pills at their ready
 * boxes, each count reserved as one digit, at the project layout's touch size
 * (w3/099).
 */
export function EnvironmentResourceSwitcherSkeleton() {
  const { t } = useTranslations();
  return (
    <div className="flex flex-wrap gap-1">
      {FILTER_KEYS.map(({ filter, key }) => (
        <ReservedButton
          key={filter}
          variant="outline"
          size="sm"
          className={TOUCH_TARGET_HEIGHT}
        >
          {t(key)} (0)
        </ReservedButton>
      ))}
    </div>
  );
}

/** The full-width search box's loading preview. */
export function EnvironmentResourceSearchSkeleton() {
  return <Skeleton className={cn("h-9 w-full", TOUCH_TARGET_HEIGHT)} />;
}

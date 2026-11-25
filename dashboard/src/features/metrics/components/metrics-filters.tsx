import { History } from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select.tsx";
import { Button } from "@/common/components/ui/button.tsx";
import { RangeSelect } from "@/features/metrics/components/range-select";
import { useMetricsRetentionBound } from "@/features/metrics/hooks/use-metrics-retention";
import type { RangeSelection } from "@/features/metrics/lib/range";
import type { EventTimelineFilter } from "@/features/events/lib/timeline";
import { useTranslations } from "@/common/hooks/use-translations";

/** The event filter and the show-timeline toggle, which travel together. */
interface MetricsEventControls {
  filter: EventTimelineFilter;
  onFilterChange: (filter: EventTimelineFilter) => void;
  timelineShown: boolean;
  onTimelineShownChange: (shown: boolean) => void;
}

interface MetricsFiltersProps {
  /** The service whose charts the toolbar ranges over. */
  resource: string;
  range: RangeSelection;
  onRangeChange: (range: RangeSelection) => void;
  /** Omitted for a cron job, whose Render page has no event timeline. */
  events?: MetricsEventControls;
}

/**
 * Render's minimal Metrics toolbar (captured live 2026-07-17, w5/m42):
 * `[event filter] [time range ▾] … [show-timeline toggle]`. Time is the only
 * page-level dimension — every chart reads the same range so the numbers
 * always agree; the per-card controls (Percentage/Total, Status Code,
 * Percentile) live on the cards they alter, exactly where Render puts them.
 * The range reaches back only as far as the workspace plan keeps metrics
 * (w7/m427): wider presets are disabled and the picker says which plans keep
 * more. Without `events` the toolbar is the range picker alone, as Render's
 * cron job Metrics page draws it (w3/103).
 */
export function MetricsFilters({
  resource,
  range,
  onRangeChange,
  events,
}: MetricsFiltersProps) {
  const { t } = useTranslations();
  const rangeBound = useMetricsRetentionBound(resource);

  if (!events) {
    return (
      <div className="flex items-center">
        <RangeSelect
          range={range}
          onRangeChange={onRangeChange}
          bound={rangeBound}
        />
      </div>
    );
  }

  return (
    // One row on a phone too: the two selects share the width beside the
    // timeline toggle, which wrapped alone onto a second row.
    <div className="flex flex-wrap items-center gap-3 max-sm:flex-nowrap max-sm:gap-2">
      <Select
        value={events.filter}
        onValueChange={(value) =>
          events.onFilterChange(value as EventTimelineFilter)
        }
      >
        <SelectTrigger
          size="sm"
          className="w-40 max-sm:w-auto max-sm:min-w-0 max-sm:flex-1"
          aria-label={t("metrics.eventsFilterLabel")}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{t("metrics.eventsFilterAll")}</SelectItem>
          <SelectItem value="deploy">
            {t("metrics.eventsFilterDeploys")}
          </SelectItem>
          <SelectItem value="lifecycle">
            {t("metrics.eventsFilterLifecycle")}
          </SelectItem>
          <SelectItem value="config">
            {t("metrics.eventsFilterConfig")}
          </SelectItem>
        </SelectContent>
      </Select>

      <RangeSelect
        range={range}
        onRangeChange={onRangeChange}
        bound={rangeBound}
        // Shares the phone row with the events select and the toggle.
        triggerClassName="max-sm:w-auto max-sm:min-w-0 max-sm:flex-1"
      />

      <Button
        size="sm"
        variant={events.timelineShown ? "default" : "outline"}
        className="ml-auto max-lg:min-w-11 pointer-coarse:min-w-11"
        aria-pressed={events.timelineShown}
        aria-label={t("metrics.eventsToggle")}
        title={t("metrics.eventsToggle")}
        onClick={() => events.onTimelineShownChange(!events.timelineShown)}
      >
        <History className="h-4 w-4" />
      </Button>
    </div>
  );
}

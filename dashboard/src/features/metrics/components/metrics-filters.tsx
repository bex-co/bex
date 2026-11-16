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

interface MetricsFiltersProps {
  /** The service whose charts the toolbar ranges over. */
  resource: string;
  range: RangeSelection;
  onRangeChange: (range: RangeSelection) => void;
  eventFilter: EventTimelineFilter;
  onEventFilterChange: (filter: EventTimelineFilter) => void;
  timelineShown: boolean;
  onTimelineShownChange: (shown: boolean) => void;
}

/**
 * Render's minimal Metrics toolbar (captured live 2026-07-17, w5/m42):
 * `[event filter] [time range ▾] … [show-timeline toggle]`. Time is the only
 * page-level dimension — every chart reads the same range so the numbers
 * always agree; the per-card controls (Percentage/Total, Status Code,
 * Percentile) live on the cards they alter, exactly where Render puts them.
 * The range reaches back only as far as the workspace plan keeps metrics
 * (w7/m427): wider presets are disabled and the picker says which plans keep
 * more.
 */
export function MetricsFilters({
  resource,
  range,
  onRangeChange,
  eventFilter,
  onEventFilterChange,
  timelineShown,
  onTimelineShownChange,
}: MetricsFiltersProps) {
  const { t } = useTranslations();
  const rangeBound = useMetricsRetentionBound(resource);

  return (
    <div className="flex flex-wrap items-center gap-3">
      <Select
        value={eventFilter}
        onValueChange={(value) =>
          onEventFilterChange(value as EventTimelineFilter)
        }
      >
        <SelectTrigger
          size="sm"
          className="w-40"
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
      />

      <Button
        size="sm"
        variant={timelineShown ? "default" : "outline"}
        className="ml-auto"
        aria-pressed={timelineShown}
        aria-label={t("metrics.eventsToggle")}
        title={t("metrics.eventsToggle")}
        onClick={() => onTimelineShownChange(!timelineShown)}
      >
        <History className="h-4 w-4" />
      </Button>
    </div>
  );
}

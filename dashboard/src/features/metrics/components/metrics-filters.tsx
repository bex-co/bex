import { Tabs, TabsList, TabsTrigger } from "@/common/components/ui/tabs.tsx";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select.tsx";
import { Button } from "@/common/components/ui/button.tsx";
import { RANGE_PRESETS, type RangePreset } from "@/features/metrics/lib/range";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";

// p50/p90/p95/p99 are quantile notation, not English words — no translation needed.
const QUANTILES = [
  { value: "0.5", label: "p50" },
  { value: "0.9", label: "p90" },
  { value: "0.95", label: "p95" },
  { value: "0.99", label: "p99" },
];

const RANGE_LABEL_KEYS: Record<RangePreset["id"], keyof typeof en> = {
  "30m": "metrics.rangeLast30Minutes",
  "1h": "metrics.rangeLastHour",
  "12h": "metrics.rangeLast12Hours",
};

interface MetricsFiltersProps {
  range: RangePreset;
  onRangeChange: (preset: RangePreset) => void;
  percentage: boolean;
  onPercentageChange: (percentage: boolean) => void;
  quantile: number;
  onQuantileChange: (quantile: number) => void;
}

/**
 * The dashboard's one filter row, above every chart (dataviz's interaction
 * rule) — every metric on the page reads the same range, so the numbers
 * always agree with each other.
 */
export function MetricsFilters({
  range,
  onRangeChange,
  percentage,
  onPercentageChange,
  quantile,
  onQuantileChange,
}: MetricsFiltersProps) {
  const { t } = useTranslations();

  return (
    <div className="flex flex-wrap items-center gap-3">
      <div className="flex gap-1">
        {RANGE_PRESETS.map((preset) => (
          <Button
            key={preset.id}
            size="sm"
            variant={preset.id === range.id ? "default" : "outline"}
            onClick={() => onRangeChange(preset)}
          >
            {t(RANGE_LABEL_KEYS[preset.id])}
          </Button>
        ))}
      </div>

      <Tabs
        value={percentage ? "percentage" : "total"}
        onValueChange={(v) => onPercentageChange(v === "percentage")}
      >
        <TabsList>
          <TabsTrigger value="percentage">{t("metrics.filterPercentage")}</TabsTrigger>
          <TabsTrigger value="total">{t("metrics.filterTotal")}</TabsTrigger>
        </TabsList>
      </Tabs>

      <Select
        value={String(quantile)}
        onValueChange={(v) => onQuantileChange(Number(v))}
      >
        <SelectTrigger size="sm" className="w-24">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {QUANTILES.map((q) => (
            <SelectItem key={q.value} value={q.value}>
              {q.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

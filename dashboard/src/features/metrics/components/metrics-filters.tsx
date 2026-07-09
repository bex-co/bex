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
  "3h": "metrics.rangeLast3Hours",
  "6h": "metrics.rangeLast6Hours",
  "12h": "metrics.rangeLast12Hours",
  "1d": "metrics.rangeLastDay",
};

// Render's Status Code dropdown offers the class presets plus the codes the
// App has actually returned (discovered via metricsFilters). "all" is the
// no-filter sentinel — Radix Select can't represent an empty-string value.
const STATUS_CODE_ALL = "all";
const STATUS_CODE_CLASSES = ["2xx", "4xx", "5xx"];

interface MetricsFiltersProps {
  range: RangePreset;
  onRangeChange: (preset: RangePreset) => void;
  percentage: boolean;
  onPercentageChange: (percentage: boolean) => void;
  quantile: number;
  onQuantileChange: (quantile: number) => void;
  /** Active status-code filter ("" = all) applied to the request metrics. */
  statusCode: string;
  onStatusCodeChange: (statusCode: string) => void;
  /** Codes the App has actually returned (metricsFilters discovery). */
  discoveredStatusCodes: string[];
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
  statusCode,
  onStatusCodeChange,
  discoveredStatusCodes,
}: MetricsFiltersProps) {
  const { t } = useTranslations();

  const statusCodeOptions = [
    ...STATUS_CODE_CLASSES,
    ...discoveredStatusCodes
      .filter((c) => !STATUS_CODE_CLASSES.includes(c))
      .sort(),
  ];

  return (
    <div className="flex flex-wrap items-center gap-3">
      <div className="flex flex-wrap gap-1">
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
          <TabsTrigger value="percentage">
            {t("metrics.filterPercentage")}
          </TabsTrigger>
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

      <Select
        value={statusCode === "" ? STATUS_CODE_ALL : statusCode}
        onValueChange={(v) =>
          onStatusCodeChange(v === STATUS_CODE_ALL ? "" : v)
        }
      >
        <SelectTrigger
          size="sm"
          className="w-40"
          aria-label={t("metrics.statusCode")}
        >
          <span className="text-muted-foreground">
            {t("metrics.statusCode")}
          </span>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={STATUS_CODE_ALL}>
            {t("metrics.statusCodeAll")}
          </SelectItem>
          {statusCodeOptions.map((code) => (
            <SelectItem key={code} value={code}>
              {code}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

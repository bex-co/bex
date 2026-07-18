import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select.tsx";
import {
  RANGE_PRESETS,
  parseRangePreset,
  type RangePreset,
} from "@/features/metrics/lib/range";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";

const RANGE_LABEL_KEYS: Record<RangePreset["id"], keyof typeof en> = {
  "30m": "metrics.rangeLast30Minutes",
  "1h": "metrics.rangeLastHour",
  "4h": "metrics.rangeLast4Hours",
  "12h": "metrics.rangeLast12Hours",
  "24h": "metrics.rangeLast24Hours",
  "2d": "metrics.rangeLast2Days",
  "7d": "metrics.rangeLast7Days",
  "14d": "metrics.rangeLast14Days",
};

interface RangeSelectProps {
  range: RangePreset;
  onRangeChange: (preset: RangePreset) => void;
  /** Overrides the default "Time range" accessible label (e.g. the Logs bar). */
  ariaLabel?: string;
}

/**
 * Render's single time-range dropdown (captured live 2026-07-17, w5/m42) —
 * shared by the Metrics toolbar and the Logs filter bar so both tabs offer
 * the same preset ladder.
 */
export function RangeSelect({
  range,
  onRangeChange,
  ariaLabel,
}: RangeSelectProps) {
  const { t } = useTranslations();

  return (
    <Select
      value={range.id}
      onValueChange={(v) => {
        const preset = parseRangePreset(v);
        if (preset) onRangeChange(preset);
      }}
    >
      <SelectTrigger
        size="sm"
        className="w-40"
        aria-label={ariaLabel ?? t("metrics.rangeLabel")}
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {RANGE_PRESETS.map((preset) => (
          <SelectItem key={preset.id} value={preset.id}>
            {t(RANGE_LABEL_KEYS[preset.id])}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

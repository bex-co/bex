import {
  RangeSelect,
  type RangeSelectProps,
} from "@/features/metrics/components/range-select";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  DEFAULT_LOG_RANGE,
  LIVE_TAIL,
  isLiveTail,
  type LogRange,
} from "../lib/log-range";

/**
 * A log explorer's time range picker (w3/m537): Render's list, Live tail
 * first, then the presets and Custom. Picking Live tail starts the tail and
 * any other range stops it, so the explorer has one control for when, as on
 * Render. Where bex's tail cannot run, Live tail is disabled, described by
 * the explorer's note saying why, and an explorer on it shows Last hour.
 */
export function LogRangeSelect({
  range,
  onRangeChange,
  liveTailDisabled,
  liveTailReasonId,
  ...props
}: Omit<RangeSelectProps, "range" | "onRangeChange" | "liveTail"> & {
  range: LogRange;
  onRangeChange: (range: LogRange) => void;
  liveTailDisabled?: boolean;
  liveTailReasonId?: string;
}) {
  const { t } = useTranslations();
  const live = isLiveTail(range) && !liveTailDisabled;
  return (
    <RangeSelect
      {...props}
      range={isLiveTail(range) ? DEFAULT_LOG_RANGE : range}
      onRangeChange={onRangeChange}
      liveTail={{
        label: t("logs.rangeLiveTail"),
        selected: live,
        disabled: liveTailDisabled,
        describedBy: liveTailReasonId,
        onSelect: () => onRangeChange(LIVE_TAIL),
      }}
    />
  );
}

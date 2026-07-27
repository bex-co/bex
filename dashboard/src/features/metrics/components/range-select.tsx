import { useState } from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select.tsx";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog.tsx";
import { Button } from "@/common/components/ui/button.tsx";
import { Input } from "@/common/components/ui/input.tsx";
import { Label } from "@/common/components/ui/label.tsx";
import {
  RANGE_PRESETS,
  isCustomRange,
  makeCustomRange,
  parseRangePreset,
  type RangePreset,
  type RangeSelection,
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
  "30d": "metrics.rangeLast30Days",
};

// The "Custom" dropdown value — a sentinel that opens the date-time picker
// instead of applying a preset (w5/m56).
const CUSTOM = "custom";

interface RangeSelectProps {
  range: RangeSelection;
  onRangeChange: (range: RangeSelection) => void;
  /** Overrides the default "Time range" accessible label (e.g. the Logs bar). */
  ariaLabel?: string;
}

/**
 * Render's single time-range dropdown (captured live 2026-07-17, w5/m42;
 * "Last 30 days" + "Custom" added w5/m56) — shared by the Metrics toolbar and
 * the Logs filter bar so both tabs offer the same preset ladder. Picking
 * "Custom" opens an absolute start/end picker; every relative preset applies
 * immediately.
 */
export function RangeSelect({
  range,
  onRangeChange,
  ariaLabel,
}: RangeSelectProps) {
  const { t } = useTranslations();
  const [pickerOpen, setPickerOpen] = useState(false);
  // Seeded when the picker opens (in the handler, not an effect) so it prefills
  // the active custom range, else the last 24 hours.
  const [draft, setDraft] = useState({ start: "", end: "" });

  function openPicker() {
    const now = new Date();
    const startAt = isCustomRange(range)
      ? new Date(range.startTime)
      : new Date(now.getTime() - 24 * 60 * 60 * 1000);
    const endAt = isCustomRange(range) ? new Date(range.endTime) : now;
    setDraft({ start: toLocalInput(startAt), end: toLocalInput(endAt) });
    setPickerOpen(true);
  }

  return (
    <>
      <Select
        value={range.id}
        onValueChange={(v) => {
          if (v === CUSTOM) {
            openPicker();
            return;
          }
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
          <SelectSeparator />
          <SelectItem value={CUSTOM}>{t("metrics.rangeCustom")}</SelectItem>
        </SelectContent>
      </Select>

      <CustomRangeDialog
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        draft={draft}
        onDraftChange={setDraft}
        onApply={(custom) => {
          onRangeChange(custom);
          setPickerOpen(false);
        }}
      />
    </>
  );
}

function CustomRangeDialog({
  open,
  onOpenChange,
  draft,
  onDraftChange,
  onApply,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  draft: { start: string; end: string };
  onDraftChange: (draft: { start: string; end: string }) => void;
  onApply: (custom: RangeSelection) => void;
}) {
  const { t } = useTranslations();
  const [error, setError] = useState<string | null>(null);

  function apply() {
    const made = makeCustomRange(
      fromLocalInput(draft.start),
      fromLocalInput(draft.end),
    );
    if (typeof made === "string") {
      setError(
        made === "order"
          ? t("metrics.rangeCustomErrorOrder")
          : t("metrics.rangeCustomErrorTooLong"),
      );
      return;
    }
    setError(null);
    onApply(made);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setError(null);
        onOpenChange(next);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("metrics.rangeCustomTitle")}</DialogTitle>
          <DialogDescription>
            {t("metrics.rangeCustomDescription")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label htmlFor="range-custom-start">
              {t("metrics.rangeCustomStart")}
            </Label>
            <Input
              id="range-custom-start"
              type="datetime-local"
              value={draft.start}
              onChange={(e) =>
                onDraftChange({ ...draft, start: e.target.value })
              }
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="range-custom-end">
              {t("metrics.rangeCustomEnd")}
            </Label>
            <Input
              id="range-custom-end"
              type="datetime-local"
              value={draft.end}
              onChange={(e) => onDraftChange({ ...draft, end: e.target.value })}
            />
          </div>
          {error !== null && (
            <p className="text-destructive text-sm">{error}</p>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("metrics.rangeCustomCancel")}
          </Button>
          <Button onClick={apply}>{t("metrics.rangeCustomApply")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** A Date → the local `YYYY-MM-DDTHH:mm` a datetime-local input expects. */
function toLocalInput(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** A datetime-local value (local wall time) → an ISO instant, or "" if blank. */
function fromLocalInput(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}

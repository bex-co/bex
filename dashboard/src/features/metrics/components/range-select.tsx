import { useState } from "react";
import { Pencil } from "lucide-react";
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
 * immediately. Re-picking the already-selected "Custom" is a no-op for a
 * controlled Select (Radix only fires onValueChange on a change), so an active
 * custom window gets its own "Edit custom range" action (w4/245).
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
  const [draft, setDraft] = useState<RangeDraft>(EMPTY_DRAFT);

  function openPicker() {
    const now = new Date();
    const startAt = isCustomRange(range)
      ? new Date(range.startTime)
      : new Date(now.getTime() - 24 * 60 * 60 * 1000);
    const endAt = isCustomRange(range) ? new Date(range.endTime) : now;
    const start = toLocalInput(startAt);
    const end = toLocalInput(endAt);
    setDraft({
      start,
      end,
      seed: {
        start,
        end,
        startIso: startAt.toISOString(),
        endIso: endAt.toISOString(),
      },
    });
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

      {isCustomRange(range) && (
        <Button size="sm" variant="outline" onClick={openPicker}>
          <Pencil aria-hidden="true" />
          {t("metrics.rangeCustomEdit")}
        </Button>
      )}

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

// The dialog's two local wall-time fields, plus what they were seeded with:
// a field still equal to its seed applies its original instant exactly, so an
// unchanged Apply never moves an accepted bound (w4/328 — the minute-only
// fields turned 18:28:10.123–18:28:50.987 into an empty, refused window).
interface RangeDraft {
  start: string;
  end: string;
  seed: { start: string; end: string; startIso: string; endIso: string } | null;
}

const EMPTY_DRAFT: RangeDraft = { start: "", end: "", seed: null };

function CustomRangeDialog({
  open,
  onOpenChange,
  draft,
  onDraftChange,
  onApply,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  draft: RangeDraft;
  onDraftChange: (draft: RangeDraft) => void;
  onApply: (custom: RangeSelection) => void;
}) {
  const { t } = useTranslations();
  const [error, setError] = useState<string | null>(null);

  function apply() {
    const seed = draft.seed;
    const made = makeCustomRange(
      seed && draft.start === seed.start
        ? seed.startIso
        : fromLocalInput(draft.start),
      seed && draft.end === seed.end ? seed.endIso : fromLocalInput(draft.end),
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

  // Every close — Radix dismissal and the footer Cancel alike — drops the
  // warning, or reopening shows it beside the reseeded valid dates (w4/279).
  function changeOpen(next: boolean) {
    if (!next) setError(null);
    onOpenChange(next);
  }

  return (
    <Dialog open={open} onOpenChange={changeOpen}>
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
              step={inputStep(draft.start)}
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
              step={inputStep(draft.end)}
              value={draft.end}
              onChange={(e) => onDraftChange({ ...draft, end: e.target.value })}
            />
          </div>
          {error !== null && (
            // Form-level span over start+end (not a single-field validation) —
            // announce once; aria-describedby on one input would mis-associate
            // (m101/t012 sibling decision).
            <p className="text-destructive text-sm" role="alert">
              {error}
            </p>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => changeOpen(false)}>
            {t("metrics.rangeCustomCancel")}
          </Button>
          <Button onClick={apply}>{t("metrics.rangeCustomApply")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** A Date → the local wall time a datetime-local input expects: `HH:mm`,
 *  extended with seconds and milliseconds only when the instant has them, so a
 *  whole-minute range looks as it always did and a precise one stays precise. */
function toLocalInput(date: Date): string {
  const pad = (n: number, width = 2) => String(n).padStart(width, "0");
  let time = `${pad(date.getHours())}:${pad(date.getMinutes())}`;
  if (date.getSeconds() || date.getMilliseconds()) {
    time += `:${pad(date.getSeconds())}`;
    if (date.getMilliseconds()) time += `.${pad(date.getMilliseconds(), 3)}`;
  }
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${time}`;
}

/** The input step that lets a datetime-local hold its value's precision (the
 *  default is 60 seconds); undefined keeps the minute picker for minute values. */
function inputStep(value: string): number | undefined {
  const seconds = value.split("T")[1]?.split(":")[2];
  if (seconds === undefined) return undefined;
  return seconds.includes(".") ? 0.001 : 1;
}

/** A datetime-local value (local wall time) → an ISO instant, or "" if blank. */
function fromLocalInput(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}

import { type ReactNode } from "react";
import { Search, SlidersHorizontal, X } from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select.tsx";
import { Input } from "@/common/components/ui/input.tsx";
import { Switch } from "@/common/components/ui/switch.tsx";
import { Label } from "@/common/components/ui/label.tsx";
import { Badge } from "@/common/components/ui/badge";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
import { Button } from "@/common/components/ui/button";
import { RangeSelect } from "@/features/metrics/components/range-select";
import { type RangeSelection } from "@/features/metrics/lib/range";
import { DEFAULT_LOG_RANGE } from "../lib/log-search";
import { useLogLabelValues } from "../hooks/use-log-label-values";
import {
  LOG_LABEL_INSTANCE,
  LOG_LABEL_LEVEL,
  LOG_LABEL_METHOD,
  LOG_LABEL_STATUS_CODE,
  LOG_TYPE_ALL,
  LOG_TYPE_APP,
  LOG_TYPE_FILTERS,
  LOG_TYPE_REQUEST,
  STRUCTURED_FILTER_KEYS,
  type LogFilters,
  type LogTypeFilter,
  type StructuredFilterKey,
} from "../types";

// Render's Logs filter row (design source .pm/w5/m6/README.md + the request/
// structured filters of .pm/w5/008, simplified to Render's minimal default in
// w7/m42): the always-visible primary toolbar carries only what Render's does —
// the type dropdown, the wide search box, the range presets, and the live
// toggle. The five structured filters (level/method/status/instance/path) live
// behind one "Filters" popover with an active-count badge, and any active
// structured filter surfaces as a removable chip so nothing is hidden when set.
// Every value bex-api honors (w3/m8); the dropdowns discover real observed
// values rather than guessing.
// LOG_TYPE_BUILD is not a user-selectable filter in the Logs tab — it is only
// used by the deploy detail page's build pane — so it is excluded from this map.
const TYPE_LABEL_KEYS: Record<
  Exclude<LogTypeFilter, "build">,
  keyof typeof en
> = {
  [LOG_TYPE_ALL]: "logs.typeAll",
  [LOG_TYPE_APP]: "logs.typeApplication",
  [LOG_TYPE_REQUEST]: "logs.typeRequest",
};

// The Radix Select "no filter" sentinel — it can't represent an empty-string
// value, so "" (all) maps to this and back at the boundary.
const ALL = "all";

// Static fallbacks so the dropdowns are usable before/without discovery (local
// dev has no store, so `logLabelValues` returns nothing). Discovered values the
// App has actually produced are merged in on top. Level/status classes mirror
// Render's fixed filter options.
const LEVEL_FALLBACK = ["debug", "info", "warning", "error"];
const METHOD_FALLBACK = ["GET", "POST", "PUT", "PATCH", "DELETE"];
const STATUS_CLASSES = ["2xx", "3xx", "4xx", "5xx"];

// mergeOptions keeps the static fallbacks first (a stable, familiar order) then
// appends discovered values not already listed, sorted — the same shape the
// metrics filter bar builds its Status Code list with.
function mergeOptions(fallback: string[], discovered: string[]): string[] {
  const extra = discovered.filter((v) => !fallback.includes(v)).sort();
  return [...fallback, ...extra];
}

// The five structured filters live behind the Filters popover — one row each
// in the popover, one removable chip each while active (list owned by types.ts).
const STRUCTURED_LABEL_KEYS: Record<StructuredFilterKey, keyof typeof en> = {
  level: "logs.levelLabel",
  method: "logs.methodLabel",
  statusCode: "logs.statusCodeLabel",
  instance: "logs.instanceLabel",
  path: "logs.pathLabel",
};

interface LogFilterBarProps {
  /** App name — scopes the `logLabelValues` discovery queries. */
  resource: string;
  filters: LogFilters;
  onChange: (patch: Partial<LogFilters>) => void;
  live: boolean;
  onLiveChange: (live: boolean) => void;
  /**
   * False when a store-only filter (request type, level/method/status/path) is
   * active — the live tail reads pod stdout and structurally can't serve those,
   * so the toggle is disabled with a hint (docs/ADR010-observability.md).
   */
  liveSupported: boolean;
  range?: RangeSelection;
  onRangeChange?: (range: RangeSelection) => void;
}

export function LogFilterBar({
  resource,
  filters,
  onChange,
  live,
  onLiveChange,
  liveSupported,
  range = DEFAULT_LOG_RANGE,
  onRangeChange = () => undefined,
}: LogFilterBarProps) {
  const { t } = useTranslations();

  // Discover real observed values for each dropdown; degrades to [] without the
  // store, where the static fallbacks carry the UI.
  const levels = mergeOptions(
    LEVEL_FALLBACK,
    useLogLabelValues(resource, LOG_LABEL_LEVEL),
  );
  const methods = mergeOptions(
    METHOD_FALLBACK,
    useLogLabelValues(resource, LOG_LABEL_METHOD),
  );
  const statuses = mergeOptions(
    STATUS_CLASSES,
    useLogLabelValues(resource, LOG_LABEL_STATUS_CODE),
  );
  const instances = mergeOptions(
    [],
    useLogLabelValues(resource, LOG_LABEL_INSTANCE),
  );

  const active = STRUCTURED_FILTER_KEYS.filter((key) => filters[key] !== "");

  return (
    <div className="flex flex-wrap items-center gap-3">
      <Select
        value={filters.type}
        onValueChange={(v) => onChange({ type: v as LogTypeFilter })}
      >
        <SelectTrigger
          size="sm"
          className="w-40"
          aria-label={t("logs.typeLabel")}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {LOG_TYPE_FILTERS.map((f) => (
            <SelectItem key={f} value={f}>
              {t(TYPE_LABEL_KEYS[f])}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <div className="relative min-w-48 flex-1">
        <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={filters.text}
          onChange={(e) => onChange({ text: e.target.value })}
          placeholder={t("logs.searchPlaceholder")}
          aria-label={t("logs.searchPlaceholder")}
          className="h-9 pl-8"
        />
      </div>

      <RangeSelect
        range={range}
        onRangeChange={onRangeChange}
        ariaLabel={t("logs.rangeLabel")}
      />

      <Popover>
        <PopoverTrigger asChild>
          <Button size="sm" variant="outline" className="gap-1.5">
            <SlidersHorizontal className="h-4 w-4" />
            {t("logs.filtersButton")}
            {active.length > 0 ? (
              <Badge
                variant="secondary"
                className="h-5 min-w-5 rounded-full px-1.5"
              >
                {active.length}
              </Badge>
            ) : null}
          </Button>
        </PopoverTrigger>
        <PopoverContent align="end" className="w-72 space-y-3">
          <SelectFilterRow
            label={t("logs.levelLabel")}
            allLabel={t("logs.levelAll")}
            value={filters.level}
            options={levels}
            onValueChange={(level) => onChange({ level })}
          />
          <SelectFilterRow
            label={t("logs.methodLabel")}
            allLabel={t("logs.methodAll")}
            value={filters.method}
            options={methods}
            onValueChange={(method) => onChange({ method })}
          />
          <SelectFilterRow
            label={t("logs.statusCodeLabel")}
            allLabel={t("logs.statusCodeAll")}
            value={filters.statusCode}
            options={statuses}
            onValueChange={(statusCode) => onChange({ statusCode })}
          />
          <SelectFilterRow
            label={t("logs.instanceLabel")}
            allLabel={t("logs.instanceAll")}
            value={filters.instance}
            options={instances}
            onValueChange={(instance) => onChange({ instance })}
          />
          <FilterRow label={t("logs.pathLabel")}>
            <Input
              value={filters.path}
              onChange={(e) => onChange({ path: e.target.value })}
              placeholder={t("logs.pathPlaceholder")}
              aria-label={t("logs.pathLabel")}
              className="h-9"
            />
          </FilterRow>
        </PopoverContent>
      </Popover>

      <div className="flex items-center gap-2">
        <Switch
          id="logs-live-toggle"
          checked={live && liveSupported}
          disabled={!liveSupported}
          onCheckedChange={onLiveChange}
        />
        <Label
          htmlFor="logs-live-toggle"
          className="cursor-pointer text-sm"
          title={liveSupported ? undefined : t("logs.liveUnsupported")}
        >
          {t("logs.live")}
        </Label>
      </div>

      {active.length > 0 ? (
        <div className="flex w-full flex-wrap items-center gap-1.5">
          {active.map((key) => {
            const label = t(STRUCTURED_LABEL_KEYS[key]);
            return (
              <Badge key={key} variant="secondary" className="gap-1 pr-1">
                {label}: {filters[key]}
                <button
                  type="button"
                  aria-label={t("logs.chipRemove", { label })}
                  onClick={() => onChange({ [key]: "" })}
                  className="rounded-full p-0.5 hover:bg-muted-foreground/20"
                >
                  <X className="h-3 w-3" />
                </button>
              </Badge>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}

// One labeled row inside the Filters popover — a visible label above the
// control (the control keeps its own aria-label, so a11y queries are stable
// whether the control sits in the bar or the popover).
function FilterRow({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="space-y-1">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      {children}
    </div>
  );
}

// One labeled discovery-fed dropdown row in the Filters popover: single-select
// with an "all" sentinel, the label doubling as the trigger's aria-label so
// a11y queries are the same whether the control sits in a bar or a popover.
function SelectFilterRow({
  label,
  allLabel,
  value,
  options,
  onValueChange,
}: {
  label: string;
  allLabel: string;
  value: string;
  options: string[];
  onValueChange: (value: string) => void;
}) {
  return (
    <FilterRow label={label}>
      <Select
        value={value === "" ? ALL : value}
        onValueChange={(v) => onValueChange(v === ALL ? "" : v)}
      >
        <SelectTrigger size="sm" className="w-full" aria-label={label}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL}>{allLabel}</SelectItem>
          {options.map((o) => (
            <SelectItem key={o} value={o}>
              {o}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FilterRow>
  );
}

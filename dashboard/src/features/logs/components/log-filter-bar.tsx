import { Search } from "lucide-react";
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
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
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
  type LogFilters,
  type LogTypeFilter,
} from "../types";

// Render's Logs filter row (design source .pm/w5/m6/README.md + the request/
// structured filters of .pm/w5/008): a type dropdown, the level/method/status/
// instance dropdowns fed by `logLabelValues`, a request-path text filter, a wide
// search box, and a live toggle. Every value bex-api honors (w3/m8); the
// dropdowns discover real observed values rather than guessing.
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
}

export function LogFilterBar({
  resource,
  filters,
  onChange,
  live,
  onLiveChange,
  liveSupported,
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

      <FilterSelect
        ariaLabel={t("logs.levelLabel")}
        allLabel={t("logs.levelAll")}
        value={filters.level}
        options={levels}
        onValueChange={(level) => onChange({ level })}
      />
      <FilterSelect
        ariaLabel={t("logs.methodLabel")}
        allLabel={t("logs.methodAll")}
        value={filters.method}
        options={methods}
        onValueChange={(method) => onChange({ method })}
      />
      <FilterSelect
        ariaLabel={t("logs.statusCodeLabel")}
        allLabel={t("logs.statusCodeAll")}
        value={filters.statusCode}
        options={statuses}
        onValueChange={(statusCode) => onChange({ statusCode })}
      />
      <FilterSelect
        ariaLabel={t("logs.instanceLabel")}
        allLabel={t("logs.instanceAll")}
        value={filters.instance}
        options={instances}
        onValueChange={(instance) => onChange({ instance })}
      />

      <Input
        value={filters.path}
        onChange={(e) => onChange({ path: e.target.value })}
        placeholder={t("logs.pathPlaceholder")}
        aria-label={t("logs.pathLabel")}
        className="h-9 w-40"
      />

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
    </div>
  );
}

// One discovery-fed filter dropdown: single-select with an "all" sentinel, the
// current value self-describing in the trigger (no separate visible label —
// aria-label carries the field name for a11y).
function FilterSelect({
  ariaLabel,
  allLabel,
  value,
  options,
  onValueChange,
}: {
  ariaLabel: string;
  allLabel: string;
  value: string;
  options: string[];
  onValueChange: (value: string) => void;
}) {
  return (
    <Select
      value={value === "" ? ALL : value}
      onValueChange={(v) => onValueChange(v === ALL ? "" : v)}
    >
      <SelectTrigger size="sm" className="w-36" aria-label={ariaLabel}>
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
  );
}

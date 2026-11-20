import { type ReactNode, type Ref, useState } from "react";
import { Search, SlidersHorizontal } from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select.tsx";
import { Input } from "@/common/components/ui/input.tsx";
import { Switch } from "@/common/components/ui/switch.tsx";
import { XIcon } from "lucide-react";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/common/components/ui/sheet";
import { useIsMobile } from "@/common/hooks/use-mobile";
import {
  LIVE_TOGGLE_CLASS,
  LOG_FILTER_BAR_CLASS,
  LOG_FILTER_TAIL_CLASS,
  LOG_SELECT_WIDTH_CLASS,
} from "./log-toolbar-layout";
import { Label } from "@/common/components/ui/label.tsx";
import { Badge } from "@/common/components/ui/badge";
import { RemovableChip } from "@/common/components/removable-chip";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import { useTranslations } from "@/common/hooks/use-translations";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import type { en } from "@/i18n";
import { Button } from "@/common/components/ui/button";
import { RangeSelect } from "@/features/metrics/components/range-select";
import { type RangeSelection } from "@/features/metrics/lib/range";
import { DEFAULT_LOG_RANGE } from "../lib/log-search";
import {
  useLogLabelDiscovery,
  type LogLabelDiscovery,
} from "../hooks/use-log-label-values";
import { useLogRetentionBound } from "../hooks/use-log-retention";
import {
  LOG_LABEL_INSTANCE,
  LOG_LABEL_LEVEL,
  LOG_LABEL_METHOD,
  LOG_LABEL_STATUS_CODE,
  LOG_LEVELS,
  LOG_TYPE_ALL,
  LOG_TYPE_APP,
  LOG_TYPE_FILTERS,
  LOG_TYPE_REQUEST,
  STRUCTURED_FILTER_KEYS,
  STRUCTURED_LABEL_KEYS,
  type LogFilters,
  type LogTypeFilter,
  LOG_METHODS,
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
//
// They are fallbacks for "we could not ask", NOT for "the answer was none" —
// see mergeOptions (w6/m131/t003).

const STATUS_CLASSES = ["2xx", "3xx", "4xx", "5xx"];

// mergeOptions keeps the static fallbacks first (a stable, familiar order) then
// appends discovered values not already listed, sorted — the same shape the
// metrics filter bar builds its Status Code list with.
//
// w6/m131/t003: when discovery has ANSWERED and the answer is "none", the
// fallbacks are dropped. A filter value the store says this App has never
// produced cannot match a single line, and offering it anyway is what made the
// request-log outage invisible — every service advertised
// GET/POST/PUT/PATCH/DELETE from this list while `label=method` returned `[]`,
// so all five selected to an empty page that read as "no traffic". Fallbacks
// still carry the UI while loading and when discovery is UNAVAILABLE (no store
// => 503), where they are a guess in place of an answer rather than a
// contradiction of one.
function mergeOptions(
  fallback: readonly string[],
  discovery: LogLabelDiscovery,
): string[] {
  if (discovery.resolved && discovery.values.length === 0) return [];
  const extra = discovery.values.filter((v) => !fallback.includes(v)).sort();
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
  /**
   * While the toggle is disabled, the id of the visible text saying why: the
   * switch's description, as a hover title can't be reached by keyboard or
   * screen reader (w4/556).
   */
  unsupportedReasonId?: string;
  range?: RangeSelection;
  onRangeChange?: (range: RangeSelection) => void;
  /**
   * False for a service that receives no public HTTP requests (a worker,
   * cron job or private service): no request log type and no request
   * filters, as Render generates request logs for web services only (w4/613).
   */
  servesRequests?: boolean;
  /** The search box, which the explorer's `/` shortcut focuses (w3/m509). */
  searchRef?: Ref<HTMLInputElement>;
  /** The explorer's Maximize and Options buttons (w3/m509), at the bar's
   *  end, in `LOG_FILTER_ACTIONS_CLASS`'s wrapper. */
  actions?: ReactNode;
}

export function LogFilterBar({
  resource,
  filters,
  onChange,
  live,
  onLiveChange,
  liveSupported,
  unsupportedReasonId,
  range = DEFAULT_LOG_RANGE,
  onRangeChange = () => undefined,
  servesRequests = true,
  searchRef,
  actions,
}: LogFilterBarProps) {
  const { t } = useTranslations();
  const hydrated = useIsHydrated();

  // Discover real observed values for each dropdown; degrades to [] without the
  // store, where the static fallbacks carry the UI.
  const levels = mergeOptions(
    LOG_LEVELS,
    useLogLabelDiscovery(resource, LOG_LABEL_LEVEL),
  );
  const methods = mergeOptions(
    LOG_METHODS,
    useLogLabelDiscovery(resource, LOG_LABEL_METHOD, { skip: !servesRequests }),
  );
  const statuses = mergeOptions(
    STATUS_CLASSES,
    useLogLabelDiscovery(resource, LOG_LABEL_STATUS_CODE, {
      skip: !servesRequests,
    }),
  );
  const instances = mergeOptions(
    [],
    useLogLabelDiscovery(resource, LOG_LABEL_INSTANCE),
  );

  const rangeBound = useLogRetentionBound(resource);

  const active = STRUCTURED_FILTER_KEYS.filter((key) => filters[key] !== "");
  const [filtersOpen, setFiltersOpen] = useState(false);
  const isPhone = useIsMobile();
  const clearButton = (
    <Button
      variant="ghost"
      size="sm"
      disabled={active.length === 0}
      onClick={() =>
        onChange(
          Object.fromEntries(
            STRUCTURED_FILTER_KEYS.map((key) => [key, ""]),
          ) as Partial<LogFilters>,
        )
      }
    >
      {t("logs.clearFilters")}
    </Button>
  );
  const doneButton = (
    <Button size="sm" className="flex-1" onClick={() => setFiltersOpen(false)}>
      {t("logs.filtersDone")}
    </Button>
  );
  const filtersTrigger = (
    <Button
      size="sm"
      variant="outline"
      className="gap-1.5 max-sm:min-w-0 max-sm:flex-1"
    >
      <SlidersHorizontal className="h-4 w-4 shrink-0" />
      <span className="truncate">{t("logs.filtersButton")}</span>
      {active.length > 0 ? (
        // On a phone the chips under the bar list each filter already.
        <Badge
          variant="secondary"
          className="h-5 min-w-5 rounded-full px-1.5 max-sm:hidden"
        >
          {active.length}
        </Badge>
      ) : null}
    </Button>
  );
  // The same fields in the phone sheet and the desktop popover.
  const filterFields = (
    <>
      <SelectFilterRow
        label={t("logs.levelLabel")}
        allLabel={t("logs.levelAll")}
        value={filters.level}
        options={levels}
        onValueChange={(level) => onChange({ level })}
      />
      {servesRequests && (
        <>
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
        </>
      )}
      <SelectFilterRow
        label={t("logs.instanceLabel")}
        allLabel={t("logs.instanceAll")}
        value={filters.instance}
        options={instances}
        onValueChange={(instance) => onChange({ instance })}
      />
      {servesRequests && (
        <>
          <FilterRow label={t("logs.pathLabel")}>
            <Input
              value={filters.path}
              onChange={(e) => onChange({ path: e.target.value })}
              placeholder={t("logs.pathPlaceholder")}
              aria-label={t("logs.pathLabel")}
              className="h-8"
            />
          </FilterRow>
          <FilterRow label={t("logs.hostLabel")}>
            <Input
              value={filters.host}
              onChange={(e) => onChange({ host: e.target.value })}
              placeholder={t("logs.hostPlaceholder")}
              aria-label={t("logs.hostLabel")}
              className="h-8"
            />
          </FilterRow>
        </>
      )}
    </>
  );

  // A phone gets two even rows in reading order, [type | search | Options]
  // then [range | Filters | Live], instead of four ragged ones that pushed
  // the first log line past the middle of the screen. It docks under the top bar
  // as the service header scrolls away, and the log fills the screen below
  // it (see useLogPanelHeight).
  return (
    <div data-log-dock="" className={LOG_FILTER_BAR_CLASS}>
      <Select
        value={filters.type}
        onValueChange={(v) => onChange({ type: v as LogTypeFilter })}
      >
        <SelectTrigger
          size="sm"
          className={LOG_SELECT_WIDTH_CLASS}
          aria-label={t("logs.typeLabel")}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {LOG_TYPE_FILTERS.filter(
            (f) => servesRequests || f !== LOG_TYPE_REQUEST,
          ).map((f) => (
            <SelectItem key={f} value={f}>
              {t(TYPE_LABEL_KEYS[f])}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <div className="relative min-w-48 flex-1 max-sm:min-w-0">
        <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          ref={searchRef}
          value={filters.text}
          // Uneditable until handlers attach: text typed into the server HTML
          // is wiped by hydration (w4/m219).
          disabled={!hydrated}
          onChange={(e) => onChange({ text: e.target.value })}
          placeholder={t("logs.searchPlaceholder")}
          aria-label={t("logs.searchPlaceholder")}
          className="h-8 pl-8"
        />
      </div>

      <RangeSelect
        range={range}
        onRangeChange={onRangeChange}
        ariaLabel={t("logs.rangeLabel")}
        bound={rangeBound}
        triggerClassName={LOG_SELECT_WIDTH_CLASS}
      />

      <div className={LOG_FILTER_TAIL_CLASS}>
        {isPhone ? (
          // A phone gets a bottom sheet: every field in view at once, rather
          // than a popover under a low trigger that showed three of six.
          <Sheet open={filtersOpen} onOpenChange={setFiltersOpen}>
            <SheetTrigger asChild>{filtersTrigger}</SheetTrigger>
            <SheetContent
              side="bottom"
              data-touch-region=""
              aria-describedby={undefined}
              // The sheet's built-in close sits apart from the title; ours shares
              // the title's row.
              className="max-h-[85dvh] gap-0 rounded-t-xl p-0 [&>button:last-child]:hidden"
            >
              <SheetHeader className="flex-row items-center justify-between border-b px-4 py-2">
                <SheetTitle>{t("logs.filtersButton")}</SheetTitle>
                <SheetClose asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={t("common.close")}
                  >
                    <XIcon className="size-4" />
                  </Button>
                </SheetClose>
              </SheetHeader>
              <div className="space-y-3 overflow-y-auto overscroll-contain px-4 py-4">
                {filterFields}
              </div>
              <SheetFooter className="flex-row border-t px-4 py-3">
                {clearButton}
                {doneButton}
              </SheetFooter>
            </SheetContent>
          </Sheet>
        ) : (
          <Popover open={filtersOpen} onOpenChange={setFiltersOpen}>
            <PopoverTrigger asChild>{filtersTrigger}</PopoverTrigger>
            <PopoverContent
              align="end"
              // A touch region of its own (it is portalled outside the
              // page's): its selects and fields take the touch height on a
              // touch screen.
              data-touch-region=""
              collisionPadding={16}
              className="max-h-[var(--radix-popover-content-available-height)] w-72 space-y-3 overflow-y-auto overscroll-contain pb-0"
              aria-label={t("logs.filtersButton")}
            >
              <p className="text-sm font-medium">{t("logs.filtersButton")}</p>
              {filterFields}
              {/* A way out that is always in reach, however far the fields
                run: pinned under them, over the fields scrolling past. */}
              <div className="sticky bottom-0 -mx-4 flex gap-2 border-t bg-popover px-4 py-3">
                {clearButton}
                {doneButton}
              </div>
            </PopoverContent>
          </Popover>
        )}

        <LiveToggle
          id="logs-live-toggle"
          checked={live && liveSupported}
          disabled={!liveSupported}
          onCheckedChange={onLiveChange}
          describedBy={unsupportedReasonId}
        />
      </div>

      {actions}

      {active.length > 0 ? (
        <div className="flex w-full flex-wrap items-center gap-1.5 max-sm:col-span-full">
          {active.map((key) => {
            const label = t(STRUCTURED_LABEL_KEYS[key]);
            return (
              <RemovableChip
                key={key}
                removeLabel={t("logs.chipRemove", { label })}
                onRemove={() => onChange({ [key]: "" })}
              >
                {label}: {filters[key]}
              </RemovableChip>
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

/** The Live switch the service and datastore log viewers share. */
export function LiveToggle({
  id,
  checked,
  disabled,
  onCheckedChange,
  describedBy,
}: {
  id: string;
  checked: boolean;
  disabled?: boolean;
  onCheckedChange: (checked: boolean) => void;
  /** The id of visible text describing the switch, e.g. why it is off. */
  describedBy?: string;
}) {
  const { t } = useTranslations();
  // One target, switch and word together: the gap between them was a dead
  // strip, and the word a separate small target.
  return (
    <Label className={LIVE_TOGGLE_CLASS}>
      <Switch
        id={id}
        checked={checked}
        disabled={disabled}
        onCheckedChange={onCheckedChange}
        aria-describedby={describedBy}
      />
      {t("logs.live")}
    </Label>
  );
}

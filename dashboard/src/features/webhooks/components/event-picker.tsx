import { useMemo, useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Checkbox } from "@/common/components/ui/checkbox";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { ScrollArea } from "@/common/components/ui/scroll-area";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  catalogEntries,
  type CatalogEvent,
} from "@/features/webhooks/event-catalog";

export interface EventPickerProps {
  /** The served vocabulary (`webhookEventTypes`) — never hardcoded. */
  eventTypes: string[];
  value: Set<string>;
  onChange: (next: Set<string>) => void;
  disabled?: boolean;
}

type TriState = boolean | "indeterminate";

function triState(selected: number, total: number): TriState {
  if (total === 0 || selected === 0) return false;
  return selected === total ? true : "indeterminate";
}

/**
 * The Render-shaped Subscribed Events picker (w1/m49, modeled on
 * dashboard.render.com/webhooks/new — docs/render-artifacts/webhooks-ui.md):
 * human-labeled events in collapsible groups with cascade, a tri-state
 * "All events" master, a search filter, and a live selected-count. Shared by
 * the create page and the webhook Settings tab. Labels/groups come from
 * event-catalog.ts; keys the catalog doesn't know render raw under "Other".
 */
export function EventPicker({
  eventTypes,
  value,
  onChange,
  disabled,
}: EventPickerProps) {
  const { t } = useTranslations();
  const [search, setSearch] = useState("");
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set());

  const entries = useMemo(() => catalogEntries(eventTypes), [eventTypes]);

  const label = (event: CatalogEvent) =>
    event.labelKey ? t(event.labelKey) : event.type;
  const groupLabel = (groupKey: string) => t(`webhooks.group.${groupKey}`);

  // Search matches an event's label or raw key, or its group's label — a
  // group-label hit keeps the whole group (Render behaves the same). The
  // vocabulary is small (17 keys today), so this filters on render, no memo.
  const needle = search.trim().toLowerCase();
  const visible = !needle
    ? entries
    : entries.flatMap((entry) => {
        if (
          entry.groupKey &&
          groupLabel(entry.groupKey).toLowerCase().includes(needle)
        ) {
          return [entry];
        }
        const events = entry.events.filter(
          (e) =>
            label(e).toLowerCase().includes(needle) ||
            e.type.toLowerCase().includes(needle),
        );
        return events.length > 0 ? [{ ...entry, events }] : [];
      });

  function setMany(types: string[], next: boolean) {
    const nextSet = new Set(value);
    for (const type of types) {
      if (next) nextSet.add(type);
      else nextSet.delete(type);
    }
    onChange(nextSet);
  }

  const allState = triState(
    eventTypes.filter((e) => value.has(e)).length,
    eventTypes.length,
  );

  function toggleCollapsed(groupKey: string) {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(groupKey)) next.delete(groupKey);
      else next.add(groupKey);
      return next;
    });
  }

  return (
    <div className="space-y-2">
      <p className="text-muted-foreground text-sm" data-testid="event-count">
        {t("webhooks.selectedCount", { count: value.size })}
      </p>
      <div className="rounded-md border">
        <div className="space-y-2 border-b p-2">
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("webhooks.searchEvents")}
            aria-label={t("webhooks.searchEvents")}
            autoComplete="off"
          />
          <Label className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 font-medium hover:bg-muted/50">
            <Checkbox
              checked={allState}
              disabled={disabled}
              onCheckedChange={() => setMany(eventTypes, allState !== true)}
              aria-label={t("webhooks.allEvents")}
            />
            {t("webhooks.allEvents")}
          </Label>
        </div>
        <ScrollArea viewportClassName="max-h-72">
          <ul className="space-y-0.5 p-2">
            {visible.map((entry) => {
              const groupKey = entry.groupKey;
              return groupKey ? (
                <GroupRows
                  key={groupKey}
                  groupTitle={groupLabel(groupKey)}
                  events={entry.events}
                  value={value}
                  disabled={disabled}
                  collapsed={collapsed.has(groupKey)}
                  onToggleCollapsed={() => toggleCollapsed(groupKey)}
                  onSetMany={setMany}
                  eventLabel={label}
                />
              ) : (
                <li key={entry.events[0].type}>
                  <EventRow
                    event={entry.events[0]}
                    eventLabel={label}
                    checked={value.has(entry.events[0].type)}
                    disabled={disabled}
                    onCheckedChange={(next) =>
                      setMany([entry.events[0].type], next)
                    }
                  />
                </li>
              );
            })}
            {visible.length === 0 ? (
              <li className="text-muted-foreground px-2 py-3 text-sm">
                {t("webhooks.searchNoMatches")}
              </li>
            ) : null}
          </ul>
        </ScrollArea>
      </div>
    </div>
  );
}

interface GroupRowsProps {
  groupTitle: string;
  events: CatalogEvent[];
  value: Set<string>;
  disabled?: boolean;
  collapsed: boolean;
  onToggleCollapsed: () => void;
  onSetMany: (types: string[], next: boolean) => void;
  eventLabel: (event: CatalogEvent) => string;
}

function GroupRows({
  groupTitle,
  events,
  value,
  disabled,
  collapsed,
  onToggleCollapsed,
  onSetMany,
  eventLabel,
}: GroupRowsProps) {
  const { t } = useTranslations();
  const types = events.map((e) => e.type);
  const state = triState(types.filter((e) => value.has(e)).length, types.length);

  return (
    <li>
      <div className="flex items-center justify-between">
        <Label className="flex flex-1 cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 font-medium hover:bg-muted/50">
          <Checkbox
            checked={state}
            disabled={disabled}
            onCheckedChange={() => onSetMany(types, state !== true)}
            aria-label={groupTitle}
          />
          {groupTitle}
        </Label>
        <Button
          type="button"
          size="icon"
          variant="ghost"
          className="size-7"
          aria-expanded={!collapsed}
          aria-label={t("webhooks.groupToggle", { group: groupTitle })}
          onClick={onToggleCollapsed}
        >
          {collapsed ? <ChevronRight /> : <ChevronDown />}
        </Button>
      </div>
      {!collapsed ? (
        <ul className="space-y-0.5">
          {events.map((event) => (
            <li key={event.type} className="ps-6">
              <EventRow
                event={event}
                eventLabel={eventLabel}
                checked={value.has(event.type)}
                disabled={disabled}
                onCheckedChange={(next) => onSetMany([event.type], next)}
              />
            </li>
          ))}
        </ul>
      ) : null}
    </li>
  );
}

interface EventRowProps {
  event: CatalogEvent;
  eventLabel: (event: CatalogEvent) => string;
  checked: boolean;
  disabled?: boolean;
  onCheckedChange: (next: boolean) => void;
}

function EventRow({
  event,
  eventLabel,
  checked,
  disabled,
  onCheckedChange,
}: EventRowProps) {
  return (
    <Label className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 hover:bg-muted/50">
      <Checkbox
        checked={checked}
        disabled={disabled}
        onCheckedChange={(v) => onCheckedChange(v === true)}
        aria-label={eventLabel(event)}
      />
      <span className={event.labelKey ? undefined : "font-mono text-sm"}>
        {eventLabel(event)}
      </span>
    </Label>
  );
}

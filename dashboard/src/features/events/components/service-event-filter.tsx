import { useMemo, useState } from "react";
import { ListFilter } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Checkbox } from "@/common/components/ui/checkbox";
import { Input } from "@/common/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import { ScrollArea } from "@/common/components/ui/scroll-area";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  SERVICE_EVENT_GROUPS,
  serviceEventLabelKey,
} from "@/features/events/service-event-catalog";

type TriState = boolean | "indeterminate";

// Group key for feed types this build has no catalog entry for.
const UNCATALOGUED_GROUP = "other";

// An uncatalogued option is labelled by its raw wire type, not by the generic
// fallback the feed row uses: ip_allow_list_changed already maps to that string
// deliberately, so reusing it here would produce two identical, unidentifiable
// checkboxes. Module-level so the memoized group list keeps a stable dep set.
function optionLabel(
  t: (key: string) => string,
  groupKey: string,
  type: string,
): string {
  return groupKey === UNCATALOGUED_GROUP ? type : t(serviceEventLabelKey(type));
}

function triState(selected: number, total: number): TriState {
  if (selected === 0) return false;
  return selected === total ? true : "indeterminate";
}

// The filter tracks HIDDEN types, not selected ones (w6/m122). With a selected
// set, "everything" had to be enumerated from the catalog, so a type the backend
// emits but the catalog does not list was excluded by the default itself — and,
// because the option list comes from the same catalog, no user action could
// re-admit it. An exclusion set inverts that: the default (empty) shows
// everything the API returned, and only a type the user actively unchecks
// disappears. `extraTypes` carries the types present in the current feed that
// the catalog does not know, so they are controllable as well as visible.
export function ServiceEventFilter({
  hidden,
  onChange,
  extraTypes = [],
}: {
  hidden: Set<string>;
  onChange: (next: Set<string>) => void;
  extraTypes?: string[];
}) {
  const { t } = useTranslations();
  const [search, setSearch] = useState("");
  const needle = search.trim().toLowerCase();
  const groups = useMemo(
    () =>
      extraTypes.length > 0
        ? [
            ...SERVICE_EVENT_GROUPS,
            { key: UNCATALOGUED_GROUP, types: extraTypes },
          ]
        : SERVICE_EVENT_GROUPS,
    [extraTypes],
  );
  const allTypes = useMemo(
    () => [...new Set(groups.flatMap((group) => group.types))],
    [groups],
  );
  const visible = useMemo(
    () =>
      groups.flatMap((group) => {
        const groupLabel = t(`services.eventsFilterGroup.${group.key}`);
        if (groupLabel.toLowerCase().includes(needle)) return [group];
        const types = group.types.filter((type) => {
          const label = optionLabel(t, group.key, type);
          return (
            label.toLowerCase().includes(needle) ||
            type.toLowerCase().includes(needle)
          );
        });
        return types.length > 0 ? [{ ...group, types }] : [];
      }),
    [groups, needle, t],
  );
  const shownCount = allTypes.filter((type) => !hidden.has(type)).length;
  const allState = triState(shownCount, allTypes.length);

  function setMany(types: string[], checked: boolean) {
    const next = new Set(hidden);
    for (const type of types) {
      if (checked) next.delete(type);
      else next.add(type);
    }
    onChange(next);
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm">
          <ListFilter className="size-3.5" aria-hidden="true" />
          {hidden.size === 0
            ? t("services.eventsFilter")
            : t("services.eventsFilterSelected", { count: shownCount })}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80 p-0">
        <div className="space-y-2 border-b p-3">
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t("services.eventsFilterSearch")}
            aria-label={t("services.eventsFilterSearch")}
            autoComplete="off"
          />
          <label className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium hover:bg-muted/50">
            <Checkbox
              checked={allState}
              onCheckedChange={() =>
                // Select-all clears the exclusion set outright rather than
                // enumerating the catalog, so it admits every type the feed
                // carries — including one this build has never heard of.
                onChange(allState === true ? new Set(allTypes) : new Set())
              }
              aria-label={t("services.eventsFilterAll")}
            />
            {t("services.eventsFilterAll")}
          </label>
        </div>
        <ScrollArea viewportClassName="max-h-80">
          <div className="space-y-4 p-3">
            {visible.map((group) => {
              const shown = group.types.filter((type) => !hidden.has(type));
              const state = triState(shown.length, group.types.length);
              const groupLabel = t(`services.eventsFilterGroup.${group.key}`);
              return (
                <section key={group.key} aria-label={groupLabel}>
                  <label className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 text-sm font-semibold hover:bg-muted/50">
                    <Checkbox
                      checked={state}
                      onCheckedChange={() =>
                        setMany(group.types, state !== true)
                      }
                      aria-label={groupLabel}
                    />
                    {groupLabel}
                  </label>
                  <div className="space-y-0.5 ps-6">
                    {group.types.map((type) => (
                      <label
                        key={type}
                        className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 text-sm hover:bg-muted/50"
                      >
                        <Checkbox
                          checked={!hidden.has(type)}
                          onCheckedChange={(checked) =>
                            setMany([type], checked === true)
                          }
                          aria-label={optionLabel(t, group.key, type)}
                        />
                        {optionLabel(t, group.key, type)}
                      </label>
                    ))}
                  </div>
                </section>
              );
            })}
            {visible.length === 0 ? (
              <p className="text-muted-foreground px-2 py-3 text-sm">
                {t("services.eventsFilterNoMatches")}
              </p>
            ) : null}
          </div>
        </ScrollArea>
      </PopoverContent>
    </Popover>
  );
}
